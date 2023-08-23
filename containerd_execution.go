package dexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/pkg/netns"
	cni "github.com/containerd/go-cni"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const randomSuffixLength = 6

type CreateTaskOptions struct {
	Image          string
	Mounts         []specs.Mount
	User           string
	Env            []string
	CommandTimeout time.Duration
	WorkingDir     string
	CommandDetails CommandDetails
}

func ByCreatingTask(opts CreateTaskOptions, logger *logrus.Entry) (Execution[Containerd], error) {
	return &createTask{opts: opts, logger: logger}, nil
}

type createTask struct {
	opts      CreateTaskOptions
	ctx       context.Context
	doneFunc  func(ctx context.Context) error
	image     containerd.Image
	container containerd.Container
	task      containerd.Task
	cmd       []string
	process   containerd.Process
	exitChan  <-chan containerd.ExitStatus
	tmpDir    string
	logger    *logrus.Entry
	net       *netns.NetNS
	cni       cni.CNI
}

func (t *createTask) create(c Containerd, cmd []string) error {
	t.cmd = cmd
	// add buffer to the command timeout
	expiration := t.opts.CommandTimeout + (5 * time.Minute)
	// the default containerd settings makes things eligible for garbage collection after 24 hours
	// since we are spinning up hundreds of thousands of tasks per day, let's set a shorter expiration
	// so we can try and be good netizens
	ctx := namespaces.WithNamespace(context.Background(), c.DefaultNamespace())
	ctx, done, err := c.WithLease(ctx, leases.WithExpiration(expiration), leases.WithRandomID())
	if err != nil {
		return fmt.Errorf("error creating containerd context: %w", err)
	}
	t.ctx = ctx
	t.doneFunc = done
	// in order for this to work the image must already be pulled into the namespace or be a
	// publicly accessible image. if we need to (re)fetch private images, we need to pass in auth to
	// the client
	image, err := c.GetImage(t.ctx, t.opts.Image)
	if err != nil {
		return fmt.Errorf("error getting image %s from namespace %s: %w", t.opts.Image, c.Client.DefaultNamespace(), err)
	}
	t.image = image

	container, err := t.createContainer(c)

	if err != nil {
		return fmt.Errorf("error creating container: %w", err)
	} else {
		logrus.Infof("successfully created container %s", container.ID())
	}
	t.container = container

	return nil
}

func (t *createTask) createContainer(c Containerd) (containerd.Container, error) {
	nerdctlArgs := t.buildCreateContainerCmd(c)
	stdout := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd := exec.Command("nerdctl", nerdctlArgs...)
	cmd.Stdout = stdout
	cmd.Stderr = stdErr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error creating container: %w", err)
	}

	containerId := strings.TrimSpace(stdout.String())
	t.logger.Infof("nerdctl created container %s", containerId)

	return c.LoadContainer(t.ctx, containerId)
}

func (t *createTask) buildCreateContainerCmd(c Containerd) []string {
	args := []string{"--namespace", c.Client.DefaultNamespace(), "create", "--name", t.generateContainerId(), "--user", t.opts.User}
	for _, m := range t.opts.Mounts {
		args = append(args, "-v", fmt.Sprintf("%s:%s", m.Source, m.Destination))
	}
	for _, e := range t.opts.Env {
		args = append(args, "-e", e)
	}
	args = append(args, t.opts.Image)
	return args
}

func (t *createTask) generateContainerId() string {
	// AA: in order to prevent errors such as being unable to re-run a command due to a failure
	// or timing issue when cleaning up a prior attempt, append a random suffix to the end to make
	// sure we can always create the container
	suffix := RandomString(randomSuffixLength)
	details := t.opts.CommandDetails
	// IDs can't have two hyphens in a row, so we use abs to generate a compliant id for the health check containers
	return fmt.Sprintf("chains-%d-%d-%d-%s", abs(details.ChainExecutorId), abs(details.ExecutorId), abs(details.ResultId), suffix)
}

func abs(v int64) int64 {
	if v >= 0 {
		return v
	}
	f := math.Abs(float64(v))
	return int64(f)
}

func (t *createTask) createUserOpts() []oci.SpecOpts {
	if t.opts.User == "" {
		return []oci.SpecOpts{}
	}
	return []oci.SpecOpts{oci.WithUser(t.opts.User), oci.WithAdditionalGIDs(t.opts.User)}
}

func (t *createTask) run(c Containerd, stdin io.Reader, stdout, stderr io.Writer) error {
	opts := []cio.Opt{cio.WithStreams(stdin, stdout, stderr)}
	task, err := t.createTask(opts...)
	if err != nil {
		return fmt.Errorf("error creating task: %w", err)
	} else {
		t.logger.Infof("successfully created task %s", task.ID())
	}

	t.task = task

	spec, err := t.createProcessSpec()
	if err != nil {
		return fmt.Errorf("error creating process spec: %w", err)
	}
	taskId := fmt.Sprintf("%s-task", t.container.ID())
	ps, err := task.Exec(t.ctx, taskId, spec, cio.NewCreator(opts...))
	if err != nil {
		return fmt.Errorf("error creating process: %w", err)
	} else {
		t.logger.Infof("successfully exec'd process %s: %v", ps.ID(), spec.Args)
	}
	t.process = ps

	// wait must always be called before start()
	t.exitChan, err = ps.Wait(t.ctx)
	if err != nil {
		return fmt.Errorf("error waiting for process: %w", err)
	} else {
		t.logger.Infof("successfully got exit channel, task pid = %d", task.Pid())
	}

	if err = ps.Start(t.ctx); err != nil {
		return fmt.Errorf("error starting process: %w", err)
	} else {
		t.logger.Infof("successfully started process %s", ps.ID())
	}
	return nil
}

func (t *createTask) createTask(opts ...cio.Opt) (containerd.Task, error) {
	if task, err := t.container.Task(t.ctx, cio.NewAttach(opts...)); err == nil {
		t.logger.Info("Using existing task")
		return task, nil
	}
	t.logger.Info("Using new task")
	return t.container.NewTask(t.ctx, cio.NewCreator(opts...))
}

func (t *createTask) createProcessSpec() (*specs.Process, error) {
	spec, err := t.container.Spec(t.ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting spec from container: %w", err)
	}

	spec.Process.Args = t.cmd
	spec.Process.Cwd = t.opts.WorkingDir
	if uid, err := strconv.ParseInt(t.opts.User, 10, 64); err == nil {
		spec.Process.User.UID = uint32(uid)
	}
	return spec.Process, nil
}

func (t *createTask) wait(c Containerd) (int, error) {
	defer t.cleanup(c)

	for {
		select {
		case exitStatus := <-t.exitChan:
			t.logger.Infof("received exit status %d from chan for ps %s", exitStatus, t.process.ID())
			return int(exitStatus.ExitCode()), exitStatus.Error()
		case <-time.After(1 * time.Second):
			status, err := t.process.Status(t.ctx)
			logrus.Infof("process status: %v", status.Status)
			if err == nil && status.Status == containerd.Stopped {
				t.logger.Warnf("process stopped but didn't receive on channel: %v", status)
				status2, err2 := t.task.Delete(t.ctx, containerd.WithProcessKill)
				t.logger.Infof("status2: %v, err2: %v", status2, err2)
				return int(status.ExitStatus), nil
			} else if err != nil {
				t.logger.Warnf("error getting process status: %v", err)
			}
		case <-t.ctx.Done():
			t.logger.Warn("context cancelled before completing process")
			return -1, context.Canceled
		}
	}

}

func (t *createTask) setEnv(env []string) error {
	if len(t.opts.Env) > 0 {
		return errors.New("dexec: Config.Env already set")
	}
	t.opts.Env = env
	return nil
}

func (t *createTask) setDir(dir string) error {
	if t.opts.WorkingDir != "" {
		return errors.New("dexec: Config.WorkingDir already set")
	}
	t.opts.WorkingDir = dir
	return nil
}

func (t *createTask) getID() string {
	return t.container.ID()
}

// kill kills the running task and cleans up any resources that were created to run it. For all intents and purposes
// kill is identical to cleanup
func (t *createTask) kill(c Containerd) error {
	return t.cleanup(c)
}

// cleanup kills any tasks that are still running, deletes them, and deletes the container that ran the task. if the
// api returns a NotFound error, the error is ignored and we will return nil. otherwise, any errors encountered during
// the cleanup operations will be returned
func (t *createTask) cleanup(Containerd) error {
	defer func() {
		if f := t.doneFunc; f != nil && t.ctx != nil {
			f(t.ctx)
		}
	}()
	if t.cni != nil && t.net != nil {
		if err := t.cni.Remove(t.ctx, t.container.ID(), t.net.GetPath()); err != nil {
			t.logger.Warnf("error removing network: %v", err)
		}
	}
	_, err := t.task.Delete(t.ctx, containerd.WithProcessKill)
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("error deleting task: %w", err)
	}
	if err = t.container.Delete(t.ctx, containerd.WithSnapshotCleanup); err == nil || errdefs.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("error deleting container: %w", err)
}

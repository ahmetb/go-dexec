package dexec

import (
	"context"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"io"
	"strconv"
	"syscall"
	"time"
)

type CreateTaskOptions struct {
	Image          string
	Mounts         []specs.Mount
	User           string
	Env            []string
	CommandTimeout time.Duration
	WorkingDir     string
}

func ByCreatingTask(opts CreateTaskOptions) (Execution[ContainerD], error) {
	return &createTask{opts: opts}, nil
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
}

func (t *createTask) create(c ContainerD, cmd []string) error {
	t.cmd = cmd
	// add buffer to the command timeout
	expiration := t.opts.CommandTimeout + (5 * time.Minute)
	// the default containerd settings makes things eligible for garbage collection after 24 hours
	// since we are spinning up hundreds of thousands of tasks per day, let's set a shorter expiration
	// so we can try and be good netizens
	ctx := namespaces.WithNamespace(context.Background(), c.Namespace)
	ctx, done, err := c.WithLease(ctx, leases.WithExpiration(expiration), leases.WithRandomID())
	if err != nil {
		return errors.Wrap(err, "error creating containerd context")
	}
	t.ctx = ctx
	t.doneFunc = done

	image, err := c.GetImage(t.ctx, t.opts.Image)
	if err != nil {
		return errors.Wrapf(err, "error pulling image %s", t.opts.Image)
	}
	t.image = image

	container, err := t.createContainer(c)
	if err != nil {
		return errors.Wrap(err, "error creating container")
	}
	t.container = container

	return nil
}

func (t *createTask) createContainer(c ContainerD) (containerd.Container, error) {
	containerId := uuid.New().String()
	snapshotName := fmt.Sprintf("%s-snapshot", containerId)

	specOpts := make([]oci.SpecOpts, 0)
	specOpts = append(specOpts, t.createUserOpts()...)
	specOpts = append(specOpts, oci.WithImageConfig(t.image), oci.WithEnv(t.opts.Env), oci.WithMounts(t.opts.Mounts))

	return c.NewContainer(
		t.ctx,
		containerId,
		containerd.WithNewSnapshot(snapshotName, t.image),
		containerd.WithNewSpec(specOpts...),
	)
}

func (t *createTask) createUserOpts() []oci.SpecOpts {
	if t.opts.User == "" {
		return []oci.SpecOpts{}
	}
	return []oci.SpecOpts{oci.WithUser(t.opts.User), oci.WithAdditionalGIDs(t.opts.User)}
}

func (t *createTask) run(c ContainerD, stdin io.Reader, stdout, stderr io.Writer) error {
	opts := []cio.Opt{cio.WithStreams(stdin, stdout, stderr)}
	task, err := t.createTask(opts...)
	if err != nil {
		return errors.Wrap(err, "error creating task")
	}

	t.task = task

	spec, err := t.createProcessSpec()
	if err != nil {
		return errors.Wrap(err, "error creating process spec")
	}
	taskId := fmt.Sprintf("%s-task", t.container.ID())
	ps, err := task.Exec(t.ctx, taskId, spec, cio.NewCreator(opts...))
	if err != nil {
		return errors.Wrap(err, "error creating process")
	}
	t.process = ps

	// wait must always be called before start()
	t.exitChan, err = ps.Wait(t.ctx)
	if err != nil {
		return errors.Wrap(err, "error waiting for process")
	}

	err = ps.Start(t.ctx)
	return errors.Wrap(err, "error starting process")
}

func (t *createTask) createTask(opts ...cio.Opt) (containerd.Task, error) {
	if task, err := t.container.Task(t.ctx, cio.NewAttach(opts...)); err == nil {
		return task, nil
	}
	return t.container.NewTask(t.ctx, cio.NewCreator(opts...))
}

func (t *createTask) createProcessSpec() (*specs.Process, error) {
	spec, err := t.container.Spec(t.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting spec from container")
	}

	spec.Process.Args = t.cmd
	spec.Process.Cwd = t.opts.WorkingDir
	uid, err := strconv.ParseInt(t.opts.User, 10, 64)
	if err == nil {
		spec.Process.User.UID = uint32(uid)
	}
	return spec.Process, nil
}

func (t *createTask) wait(c ContainerD) (int, error) {
	defer t.cleanup(c)

	exitStatus := <-t.exitChan
	return int(exitStatus.ExitCode()), exitStatus.Error()
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

func (t *createTask) kill(c ContainerD) error {
	err := t.task.Kill(t.ctx, syscall.SIGKILL, containerd.WithKillAll)
	if err != nil {
		return errors.Wrap(err, "error killing task")
	}

	return t.cleanup(c)
}

func (t *createTask) cleanup(ContainerD) error {
	defer t.doneFunc(t.ctx)
	_, err := t.task.Delete(t.ctx)
	if err != nil {
		return errors.Wrap(err, "error deleting task")
	}
	return errors.Wrap(t.container.Delete(t.ctx, containerd.WithSnapshotCleanup), "error deleting container")
}

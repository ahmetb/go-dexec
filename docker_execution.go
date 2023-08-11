package dexec

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"

	"github.com/fsouza/go-dockerclient"
)

type createContainer struct {
	opt docker.CreateContainerOptions
	cmd []string
	id  string // created container id
	cw  docker.CloseWaiter
}

// ByCreatingContainer is the execution strategy where a new container with specified
// options is created to execute the command.
//
// The container will be created and started with Cmd.Start and will be deleted
// before Cmd.Wait returns.
func ByCreatingContainer(opts docker.CreateContainerOptions) (Execution[Docker], error) {
	if opts.Config == nil {
		return nil, errors.New("dexec: Config is nil")
	}
	return &createContainer{opt: opts}, nil
}

func (c *createContainer) setEnv(env []string) error {
	if len(c.opt.Config.Env) > 0 {
		return errors.New("dexec: Config.Env already set")
	}
	c.opt.Config.Env = env
	return nil
}

func (c *createContainer) setDir(dir string) error {
	if c.opt.Config.WorkingDir != "" {
		return errors.New("dexec: Config.WorkingDir already set")
	}
	c.opt.Config.WorkingDir = dir
	return nil
}

func (c *createContainer) create(d Docker, cmd []string) error {
	c.cmd = cmd

	if len(c.opt.Config.Cmd) > 0 {
		return errors.New("dexec: Config.Cmd already set")
	}
	if len(c.opt.Config.Entrypoint) > 0 {
		return errors.New("dexec: Config.Entrypoint already set")
	}

	c.opt.Config.AttachStdin = true
	c.opt.Config.AttachStdout = true
	c.opt.Config.AttachStderr = true
	c.opt.Config.OpenStdin = true
	c.opt.Config.StdinOnce = true
	c.opt.Config.Cmd = nil        // clear cmd
	c.opt.Config.Entrypoint = cmd // set new entrypoint

	container, err := d.Client.CreateContainer(c.opt)
	if err != nil {
		return fmt.Errorf("dexec: failed to create container: %v", err)
	}

	c.id = container.ID
	return nil
}

func (c *createContainer) run(d Docker, stdin io.Reader, stdout, stderr io.Writer) error {
	if c.id == "" {
		return errors.New("dexec: container is not created")
	}
	if err := d.Client.StartContainer(c.id, nil); err != nil {
		return fmt.Errorf("dexec: failed to start container:  %v", err)
	}

	i, err := d.Client.InspectContainerWithOptions(docker.InspectContainerOptions{Context: context.Background(), ID: c.id, Size: false})
	if err == nil {
		j, err := json.Marshal(i)
		if err == nil {
			logrus.Infof("container definition: %s", string(j))
		} else {
			logrus.Warnf("error converting definition to json: %v", err)
		}
	} else {
		logrus.Warnf("error getting container definition: %v", err)
	}

	opts := docker.AttachToContainerOptions{
		Container:    c.id,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		InputStream:  stdin,
		OutputStream: stdout,
		ErrorStream:  stderr,
		Stream:       true,
		Logs:         true, // include produced output so far
	}
	cw, err := d.Client.AttachToContainerNonBlocking(opts)
	if err != nil {
		return fmt.Errorf("dexec: failed to attach container: %v", err)
	}
	c.cw = cw
	return nil
}

func (c *createContainer) wait(d Docker) (exitCode int, err error) {
	del := func() error { return d.RemoveContainer(docker.RemoveContainerOptions{ID: c.id, Force: true}) }
	defer del()
	if c.cw == nil {
		return -1, errors.New("dexec: container is not attached")
	}
	if err = c.cw.Wait(); err != nil {
		return -1, fmt.Errorf("dexec: attach error: %v", err)
	}
	ec, err := d.WaitContainer(c.id)
	if err != nil {
		return -1, fmt.Errorf("dexec: cannot wait for container: %v", err)
	}
	if err := del(); err != nil {
		return -1, fmt.Errorf("dexec: error deleting container: %v", err)
	}
	return ec, nil
}

func (c *createContainer) getID() string {
	return c.id
}

func (c *createContainer) kill(d Docker) error {
	var nsc *docker.NoSuchContainer
	var cnr *docker.ContainerNotRunning
	err := d.StopContainer(c.getID(), 1)
	// if container doesn't exist or already is killed
	// do not return an error
	if err == nil || errors.As(err, &nsc) || errors.As(err, &cnr) {
		return nil
	}
	return errors.Wrap(err, "error stopping container")
}

func (c *createContainer) cleanup(d Docker) error {
	containerId := c.getID()
	var nsc *docker.NoSuchContainer
	err := d.StopContainer(containerId, 1)
	// if container doesn't exist we have nothing else to do
	if errors.As(err, &nsc) {
		return nil
	}
	var cnr *docker.ContainerNotRunning
	if err != nil && !errors.As(err, &cnr) {
		return errors.Wrap(err, "error stopping container")
	}

	return errors.Wrap(d.RemoveContainer(docker.RemoveContainerOptions{ID: containerId}), "error removing container")
}

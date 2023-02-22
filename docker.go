package dexec

import (
	"bytes"
	"errors"
	docker "github.com/fsouza/go-dockerclient"
	"io"
	"io/ioutil"
)

// Docker contains connection to Docker API.
// Use github.com/fsouza/go-dockerclient to initialize *docker.Client.
type Docker struct {
	*docker.Client
}

// Command returns the Cmd struct to execute the named program with given
// arguments using specified execution method.
//
// For each new Cmd, you should create a new instance for "method" argument.
func (d Docker) Command(method Execution[Docker], name string, arg ...string) *DockerCmd {
	return &DockerCmd{
		GenericCmd: GenericCmd{
			Path: name,
			Args: arg,
		},
		Method: method,
		docker: d,
	}
}

// DockerCmd represents an external command being prepared or run.
//
// A DockerCmd cannot be reused after calling its Run, Output or CombinedOutput
// methods.
type DockerCmd struct {
	GenericCmd
	// Method provides the execution strategy for the context of the Cmd.
	// An instance of Method should not be reused between Cmds.
	Method Execution[Docker]

	docker         Docker
	closeAfterWait []io.Closer
}

// Start starts the specified command but does not wait for it to complete.
func (c *DockerCmd) Start() error {
	if c.Dir != "" {
		if err := c.Method.setDir(c.Dir); err != nil {
			return err
		}
	}
	if c.Env != nil {
		if err := c.Method.setEnv(c.Env); err != nil {
			return err
		}
	}

	if c.started {
		return errors.New("dexec: already started")
	}
	c.started = true

	if c.Stdin == nil {
		c.Stdin = empty
	}
	if c.Stdout == nil {
		c.Stdout = ioutil.Discard
	}
	if c.Stderr == nil {
		c.Stderr = ioutil.Discard
	}

	cmd := append([]string{c.Path}, c.Args...)
	if err := c.Method.create(c.docker, cmd); err != nil {
		return err
	}
	if err := c.Method.run(c.docker, c.Stdin, c.Stdout, c.Stderr); err != nil {
		return err
	}
	return nil
}

// Wait waits for the command to exit. It must have been started by Start.
//
// If the container exits with a non-zero exit code, the error is of type
// *ExitError. Other error types may be returned for I/O problems and such.
//
// Different than os/exec.Wait, this method will not release any resources
// associated with Cmd (such as file handles).
func (c *DockerCmd) Wait() error {
	defer closeFds(c.closeAfterWait)
	if !c.started {
		return errors.New("dexec: not started")
	}
	ec, err := c.Method.wait(c.docker)
	if err != nil {
		return err
	}
	if ec != 0 {
		return &ExitError{ExitCode: ec}
	}
	return nil
}

// Run starts the specified command and waits for it to complete.
//
// If the command runs successfully and copying streams are done as expected,
// the error is nil.
//
// If the container exits with a non-zero exit code, the error is of type
// *ExitError. Other error types may be returned for I/O problems and such.
func (c *DockerCmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// CombinedOutput runs the command and returns its combined standard output and
// standard error.
//
// Docker API does not have strong guarantees over ordering of messages. For instance:
//     >&1 echo out; >&2 echo err
// may result in "out\nerr\n" as well as "err\nout\n" from this method.
func (c *DockerCmd) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("dexec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("dexec: Stderr already set")
	}
	var b bytes.Buffer
	c.Stdout, c.Stderr = &b, &b
	err := c.Run()
	return b.Bytes(), err
}

// Output runs the command and returns its standard output.
//
// If the container exits with a non-zero exit code, the error is of type
// *ExitError. Other error types may be returned for I/O problems and such.
//
// If c.Stderr was nil, Output populates ExitError.Stderr.
func (c *DockerCmd) Output() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("dexec: Stdout already set")
	}
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout

	captureErr := c.Stderr == nil
	if captureErr {
		c.Stderr = &stderr
	}
	err := c.Run()
	if err != nil && captureErr {
		if ee, ok := err.(*ExitError); ok {
			ee.Stderr = stderr.Bytes()
		}
	}
	return stdout.Bytes(), err
}

// StdinPipe returns a pipe that will be connected to the command's standard input
// when the command starts.
//
// Different than os/exec.StdinPipe, returned io.WriteCloser should be closed by user.
func (c *DockerCmd) StdinPipe() (io.WriteCloser, error) {
	if c.Stdin != nil {
		return nil, errors.New("dexec: Stdin already set")
	}
	pr, pw := io.Pipe()
	c.Stdin = pr
	return pw, nil
}

// StdoutPipe returns a pipe that will be connected to the command's standard output when
// the command starts.
//
// Wait will close the pipe after seeing the command exit or in error conditions.
func (c *DockerCmd) StdoutPipe() (io.ReadCloser, error) {
	if c.Stdout != nil {
		return nil, errors.New("dexec: Stdout already set")
	}
	pr, pw := io.Pipe()
	c.Stdout = pw
	c.closeAfterWait = append(c.closeAfterWait, pw)
	return pr, nil
}

// StderrPipe returns a pipe that will be connected to the command's standard error when
// the command starts.
//
// Wait will close the pipe after seeing the command exit or in error conditions.
func (c *DockerCmd) StderrPipe() (io.ReadCloser, error) {
	if c.Stderr != nil {
		return nil, errors.New("dexec: Stderr already set")
	}
	pr, pw := io.Pipe()
	c.Stderr = pw
	c.closeAfterWait = append(c.closeAfterWait, pw)
	return pr, nil
}

// GetPID will return the container ID of the Cmd's running container.  This is useful
// for when we need to kill the process before completion or store its container ID
func (c *DockerCmd) GetPID() string {
	if c.started {
		return c.Method.getID()
	}

	return ""
}

// Kill will stop a running container
func (c *DockerCmd) Kill() error {
	if c.started {
		return c.Method.kill(c.docker)
	}

	return nil
}

func (c *DockerCmd) SetDir(dir string) {
	c.Dir = dir
}

func (c *DockerCmd) SetStderr(writer io.Writer) {
	c.Stderr = writer
}

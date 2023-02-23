package dexec

import (
	"bytes"
	"errors"
	"github.com/containerd/containerd"
	"io"
	"io/ioutil"
)

type ContainerD struct {
	*containerd.Client
	Namespace string
}

type ContainerDCmd struct {
	GenericCmd
	Method     Execution[ContainerD]
	containerD ContainerD
}

func (c ContainerD) Command(method Execution[ContainerD], name string, arg ...string) *ContainerDCmd {
	return &ContainerDCmd{
		GenericCmd: GenericCmd{
			Path: name,
			Args: arg,
		},
		Method:     method,
		containerD: c,
	}
}

func (c *ContainerDCmd) Start() error {
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
	if err := c.Method.create(c.containerD, cmd); err != nil {
		return err
	}
	if err := c.Method.run(c.containerD, c.Stdin, c.Stdout, c.Stderr); err != nil {
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
func (c *ContainerDCmd) Wait() error {
	defer closeFds(c.closeAfterWait)
	if !c.started {
		return errors.New("dexec: not started")
	}
	ec, err := c.Method.wait(c.containerD)
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
func (c *ContainerDCmd) Run() error {
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
func (c *ContainerDCmd) CombinedOutput() ([]byte, error) {
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
func (c *ContainerDCmd) Output() ([]byte, error) {
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
func (c *ContainerDCmd) StdinPipe() (io.WriteCloser, error) {
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
func (c *ContainerDCmd) StdoutPipe() (io.ReadCloser, error) {
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
func (c *ContainerDCmd) StderrPipe() (io.ReadCloser, error) {
	if c.Stderr != nil {
		return nil, errors.New("dexec: Stderr already set")
	}
	pr, pw := io.Pipe()
	c.Stderr = pw
	c.closeAfterWait = append(c.closeAfterWait, pw)
	return pr, nil
}

// GetPID will return the container ID of the Cmd's running container.  This is useful
// for when we need to cleanup the process before completion or store its container ID
func (c *ContainerDCmd) GetPID() string {
	if c.started {
		return c.Method.getID()
	}

	return ""
}

func (c *ContainerDCmd) SetDir(dir string) {
	c.Dir = dir
}

func (c *ContainerDCmd) SetStderr(writer io.Writer) {
	c.Stderr = writer
}

// Kill will stop a running container
func (c *ContainerDCmd) Kill() error {
	if c.started {
		return c.Method.kill(c.containerD)
	}

	return nil
}

func (c *ContainerDCmd) Cleanup() error {
	return c.Method.cleanup(c.containerD)
}

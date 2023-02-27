package dexec

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
)

type Cmd interface {
	// StdoutPipe returns a pipe that will be connected to the command's standard output when
	// the command starts.
	//
	// Wait will close the pipe after seeing the command exit or in error conditions.
	StdoutPipe() (io.ReadCloser, error)
	// StderrPipe returns a pipe that will be connected to the command's standard error when
	// the command starts.
	//
	// Wait will close the pipe after seeing the command exit or in error conditions.
	StderrPipe() (io.ReadCloser, error)
	// SetStderr sets the stderr writer
	SetStderr(writer io.Writer)
	// StdinPipe returns a pipe that will be connected to the command's standard input
	// when the command starts.
	//
	// Different than os/exec.StdinPipe, returned io.WriteCloser should be closed by user.
	StdinPipe() (io.WriteCloser, error)
	// GetPID will return a unique identifier for the running command. The meaning of the identifier
	// may be implementation specific
	GetPID() string
	// Start starts the specified command but does not wait for it to complete.
	Start() error
	// Wait waits for the command to exit. It must have been started by Start.
	//
	// If the container exits with a non-zero exit code, the error is of type
	// *ExitError. Other error types may be returned for I/O problems and such.
	//
	// Different than os/exec.Wait, this method will not release any resources
	// associated with Cmd (such as file handles).
	Wait() error
	// Kill will stop a running command
	Kill() error
	// Run starts the specified command and waits for it to complete.
	//
	// If the command runs successfully and copying streams are done as expected,
	// the error is nil.
	//
	// If the container exits with a non-zero exit code, the error is of type
	// *ExitError. Other error types may be returned for I/O problems and such.
	Run() error
	// Output runs the command and returns its standard output.
	//
	// If the container exits with a non-zero exit code, the error is of type
	// *ExitError. Other error types may be returned for I/O problems and such.
	//
	// If c.Stderr was nil, Output populates ExitError.Stderr.
	Output() ([]byte, error)
	// CombinedOutput runs the command and returns its combined standard output and
	// standard error.
	//
	// Docker API does not have strong guarantees over ordering of messages. For instance:
	//     >&1 echo out; >&2 echo err
	// may result in "out\nerr\n" as well as "err\nout\n" from this method.
	CombinedOutput() ([]byte, error)
	// SetDir sets the working directory for the command
	SetDir(dir string)
	// Cleanup cleans up any resources that were created for the command
	Cleanup() error
}

type GenericCmd[T ContainerClient] struct {
	// Path is the path or name of the command in the container.
	Path string

	// Arguments to the command in the container, excluding the command
	// name as the first argument.
	Args []string

	// Env is environment variables to the command. If Env is nil, Run will use
	// Env specified on Method or pre-built container image.
	Env []string

	// Dir specifies the working directory of the command. If Dir is the empty
	// string, Run uses Dir specified on Method or pre-built container image.
	Dir string

	// Stdin specifies the process's standard input.
	// If Stdin is nil, the process reads from the null device (os.DevNull).
	//
	// Run will not close the underlying handle if the Reader is an *os.File
	// differently than os/exec.
	Stdin io.Reader

	// Stdout and Stderr specify the process's standard output and error.
	// If either is nil, they will be redirected to the null device (os.DevNull).
	//
	// Run will not close the underlying handles if they are *os.File differently
	// than os/exec.
	Stdout         io.Writer
	Stderr         io.Writer
	started        bool
	Method         Execution[T]
	closeAfterWait []io.Closer
	client         T
}

func (g *GenericCmd[T]) Start() error {
	if g.Dir != "" {
		if err := g.Method.setDir(g.Dir); err != nil {
			return err
		}
	}
	if g.Env != nil {
		if err := g.Method.setEnv(g.Env); err != nil {
			return err
		}
	}

	if g.started {
		return errors.New("dexec: already started")
	}
	g.started = true

	if g.Stdin == nil {
		g.Stdin = empty
	}
	if g.Stdout == nil {
		g.Stdout = ioutil.Discard
	}
	if g.Stderr == nil {
		g.Stderr = ioutil.Discard
	}

	cmd := append([]string{g.Path}, g.Args...)
	if err := g.Method.create(g.client, cmd); err != nil {
		return err
	}
	if err := g.Method.run(g.client, g.Stdin, g.Stdout, g.Stderr); err != nil {
		return err
	}
	return nil
}

func (g *GenericCmd[T]) Wait() error {
	defer closeFds(g.closeAfterWait)
	if !g.started {
		return errors.New("dexec: not started")
	}
	ec, err := g.Method.wait(g.client)
	if err != nil {
		return err
	}
	if ec != 0 {
		return &ExitError{ExitCode: ec}
	}
	return nil
}

func (g *GenericCmd[T]) Run() error {
	if err := g.Start(); err != nil {
		return err
	}
	return g.Wait()
}

func (g *GenericCmd[T]) CombinedOutput() ([]byte, error) {
	if g.Stdout != nil {
		return nil, errors.New("dexec: Stdout already set")
	}
	if g.Stderr != nil {
		return nil, errors.New("dexec: Stderr already set")
	}
	var b bytes.Buffer
	g.Stdout, g.Stderr = &b, &b
	err := g.Run()
	return b.Bytes(), err
}

func (g *GenericCmd[T]) Output() ([]byte, error) {
	if g.Stdout != nil {
		return nil, errors.New("dexec: Stdout already set")
	}
	var stdout, stderr bytes.Buffer
	g.Stdout = &stdout

	captureErr := g.Stderr == nil
	if captureErr {
		g.Stderr = &stderr
	}
	err := g.Run()
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
func (g *GenericCmd[T]) StdinPipe() (io.WriteCloser, error) {
	if g.Stdin != nil {
		return nil, errors.New("dexec: Stdin already set")
	}
	pr, pw := io.Pipe()
	g.Stdin = pr
	return pw, nil
}

// StdoutPipe returns a pipe that will be connected to the command's standard output when
// the command starts.
//
// Wait will close the pipe after seeing the command exit or in error conditions.
func (g *GenericCmd[T]) StdoutPipe() (io.ReadCloser, error) {
	if g.Stdout != nil {
		return nil, errors.New("dexec: Stdout already set")
	}
	pr, pw := io.Pipe()
	g.Stdout = pw
	g.closeAfterWait = append(g.closeAfterWait, pw)
	return pr, nil
}

// StderrPipe returns a pipe that will be connected to the command's standard error when
// the command starts.
//
// Wait will close the pipe after seeing the command exit or in error conditions.
func (g *GenericCmd[T]) StderrPipe() (io.ReadCloser, error) {
	if g.Stderr != nil {
		return nil, errors.New("dexec: Stderr already set")
	}
	pr, pw := io.Pipe()
	g.Stderr = pw
	g.closeAfterWait = append(g.closeAfterWait, pw)
	return pr, nil
}

// GetPID will return the container ID of the Cmd's running container.  This is useful
// for when we need to cleanup the process before completion or store its container ID
func (g *GenericCmd[T]) GetPID() string {
	if g.started {
		return g.Method.getID()
	}

	return ""
}

func (g *GenericCmd[T]) SetDir(dir string) {
	g.Dir = dir
}

func (g *GenericCmd[T]) SetStderr(writer io.Writer) {
	g.Stderr = writer
}

// Kill will stop a running container
func (g *GenericCmd[T]) Kill() error {
	if g.started {
		return g.Method.kill(g.client)
	}

	return nil
}

func (g *GenericCmd[T]) Cleanup() error {
	return g.Method.cleanup(g.client)
}

func closeFds(l []io.Closer) {
	for _, fd := range l {
		fd.Close()
	}
}

type emptyReader struct{}

func (r *emptyReader) Read(b []byte) (int, error) { return 0, io.EOF }

var empty = &emptyReader{}

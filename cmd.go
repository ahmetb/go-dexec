package dexec

import (
	"io"
)

type Cmd interface {
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	SetStderr(writer io.Writer)
	GetPID() string
	Start() error
	Wait() error
	Kill() error
	Run() error
	Output() ([]byte, error)
	CombinedOutput() ([]byte, error)
}

// Command returns the Cmd struct to execute the named program with given
// arguments using specified execution method.
//
// For each new Cmd, you should create a new instance for "method" argument.
func (d Docker) Command(method Execution[Docker], name string, arg ...string) *DockerCmd {
	return &DockerCmd{Method: method, Path: name, Args: arg, docker: d}
}

func closeFds(l []io.Closer) {
	for _, fd := range l {
		fd.Close()
	}
}

type emptyReader struct{}

func (r *emptyReader) Read(b []byte) (int, error) { return 0, io.EOF }

var empty = &emptyReader{}

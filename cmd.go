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
	SetDir(dir string)
	Cleanup() error
}

type GenericCmd struct {
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
	closeAfterWait []io.Closer
}

func closeFds(l []io.Closer) {
	for _, fd := range l {
		fd.Close()
	}
}

type emptyReader struct{}

func (r *emptyReader) Read(b []byte) (int, error) { return 0, io.EOF }

var empty = &emptyReader{}

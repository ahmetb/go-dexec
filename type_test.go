package dexec_test

import (
	"io"
	osexec "os/exec"
	"testing"

	"github.com/Workiva/go-dexec"
)

// cmd ensures interface compatibility between os/exec.Cmd and dexec.Cmd.
type cmd interface {
	CombinedOutput() ([]byte, error)
	Output() ([]byte, error)
	Run() error
	Start() error
	StderrPipe() (io.ReadCloser, error)
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	Wait() error
}

func TestOSExecCommandMatchesInterface(_ *testing.T) {
	var c cmd
	v := new(osexec.Cmd)
	c = v // compile error
	_ = c
}

func TestDexecCommandMatchesInterface(_ *testing.T) {
	var c cmd
	v := new(dexec.DockerCmd)
	c = v // compile error
	_ = c
}

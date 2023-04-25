package dexec

import (
	"github.com/containerd/containerd"
)

type Containerd struct {
	*containerd.Client
	Namespace string
}

type ContainerdCmd struct {
	*GenericCmd[Containerd]
}

func (c Containerd) Command(method Execution[Containerd], name string, arg ...string) *ContainerdCmd {
	return &ContainerdCmd{
		GenericCmd: &GenericCmd[Containerd]{
			Path:   name,
			Args:   arg,
			Method: method,
			client: c,
		},
	}
}

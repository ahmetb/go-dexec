package dexec

import (
	"github.com/containerd/containerd"
)

type ContainerD struct {
	*containerd.Client
	Namespace string
}

type ContainerDCmd struct {
	*GenericCmd[ContainerD]
}

func (c ContainerD) Command(method Execution[ContainerD], name string, arg ...string) *ContainerDCmd {
	return &ContainerDCmd{
		GenericCmd: &GenericCmd[ContainerD]{
			Path:   name,
			Args:   arg,
			Method: method,
			client: c,
		},
	}
}

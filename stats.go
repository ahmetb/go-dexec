package dexec

import "github.com/containerd/containerd"

type Stats struct {
	Running          int
	Created          int
	Stopped          int
	Paused           int
	Pausing          int
	Unknown          int
	DeadlineExceeded int
	Errors           int
}

func GetStats(client interface{}) (Stats, error) {
	switch c := client.(type) {
	case *containerd.Client:
		return getContainerdStats(c)
	default:
		return Stats{}, nil
	}
}

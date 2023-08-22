package dexec

import (
	"context"
	"errors"
	"fmt"
	"github.com/containerd/containerd"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/opencontainers/runtime-spec/specs-go"
	"strings"
)

func Command(client interface{}, config Config) Cmd {
	switch c := client.(type) {
	case *docker.Client:
		dc := Docker{Client: c}
		execution := getDockerExecution(config)
		return dc.Command(execution, config.TaskConfig.Executable, config.TaskConfig.Args...)
	case *containerd.Client:
		if c.DefaultNamespace() == "" {
			panic(errors.New("containerd client must have default namespace set"))
		}
		cdc := Containerd{Client: c}
		execution := getContainerdExecution(config)
		return cdc.Command(execution, config.TaskConfig.Executable, config.TaskConfig.Args...)
	default:
		panic(fmt.Errorf("unsupported client type: %v", c))
	}
}

func getDockerExecution(config Config) Execution[Docker] {
	mounts := filterMounts[docker.HostMount](config.ContainerConfig.Mounts)
	exec, _ := ByCreatingContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image:        config.ContainerConfig.Image,
			AttachStdout: true,
			AttachStderr: true,
			User:         config.ContainerConfig.User,
			Env:          config.ContainerConfig.Env,
		},
		HostConfig: &docker.HostConfig{
			DNS:        config.NetworkConfig.DNS,
			DNSSearch:  config.NetworkConfig.DNSSearch,
			DNSOptions: config.NetworkConfig.DNSOptions,
			Mounts:     convertMounts[docker.HostMount](mounts),
		},
		Context: context.Background(),
	})
	return exec
}

func getContainerdExecution(config Config) Execution[Containerd] {
	mounts := filterMounts[specs.Mount](config.ContainerConfig.Mounts)
	exec, _ := ByCreatingTask(CreateTaskOptions{
		Image:          config.ContainerConfig.Image,
		Mounts:         convertMounts[specs.Mount](mounts),
		User:           config.ContainerConfig.User,
		Env:            config.ContainerConfig.Env,
		CommandTimeout: config.TaskConfig.Timeout,
		WorkingDir:     config.TaskConfig.WorkingDir,
		CommandDetails: config.CommandDetails,
	}, config.Logger)
	return exec
}

type mountable interface {
	docker.HostMount | specs.Mount
}

func convertMounts[T mountable](ms []Mount) []T {
	mounts := make([]T, len(ms))
	for i, mount := range ms {
		mounts[i] = convertMount[T](mount)
	}
	return mounts
}

func filterMounts[T mountable](ms []Mount) []Mount {
	var t T
	switch any(&t).(type) {
	case *docker.HostMount:
		mounts := make([]Mount, 0)
		for _, m := range ms {
			if !strings.Contains(m.Destination, "resolv.conf") {
				mounts = append(mounts, m)
			}
		}
		return mounts
	default:
		return ms
	}
}

func convertMount[T mountable](m Mount) T {
	var res T
	switch v := any(&res).(type) {
	case *docker.HostMount:
		*v = docker.HostMount{
			Type:   m.Type,
			Source: m.Source,
			Target: m.Destination,
		}
	case *specs.Mount:
		*v = specs.Mount{
			Type:        m.Type,
			Source:      m.Source,
			Destination: m.Destination,
			Options:     m.Options,
		}
	}
	return res
}

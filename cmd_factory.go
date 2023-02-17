package dexec

import (
	"context"
	"fmt"
	"github.com/containerd/containerd"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func Command(client interface{}, config Config) Cmd {
	switch c := client.(type) {
	case *docker.Client:
		dc := Docker{Client: c}
		execution := getDockerExecution(config)
		return dc.Command(execution, config.TaskConfig.Executable, config.TaskConfig.Args...)
	case *containerd.Client:
		cdc := ContainerD{Client: c}
		execution := getContainerDExecution(config)
		return cdc.Command(execution, config.TaskConfig.Executable, config.TaskConfig.Args...)
	default:
		panic(fmt.Errorf("unsupported client type: %v", c))
	}
}

func getDockerExecution(config Config) Execution[Docker] {
	exec, _ := ByCreatingContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image:        config.ContainerConfig.Image,
			AttachStdout: true,
			AttachStderr: true,
			User:         config.ContainerConfig.User,
			Env:          config.ContainerConfig.Env,
		},
		HostConfig: &docker.HostConfig{
			DNS:    config.NetworkConfig.DNS,
			Mounts: convertDockerMounts(config.ContainerConfig.Mounts),
		},
		Context: context.Background(),
	})
	return exec
}

func getContainerDExecution(config Config) Execution[ContainerD] {
	exec, _ := ByCreatingTask(CreateTaskOptions{
		Image:          config.ContainerConfig.Image,
		Mounts:         convertContainerDMounts(config.ContainerConfig.Mounts),
		User:           config.ContainerConfig.User,
		Env:            config.ContainerConfig.Env,
		CommandTimeout: config.TaskConfig.Timeout,
		WorkingDir:     config.TaskConfig.WorkingDir,
	})
	return exec
}

func convertContainerDMounts(ms []Mount) []specs.Mount {
	mounts := make([]specs.Mount, len(ms))
	for i, mount := range ms {
		mounts[i] = specs.Mount{
			Type:        mount.Type,
			Source:      mount.Source,
			Destination: mount.Destination,
			Options:     mount.Options,
		}
	}
	return mounts
}

func convertDockerMounts(ms []Mount) []docker.HostMount {
	mounts := make([]docker.HostMount, len(ms))
	for i, mount := range ms {
		mounts[i] = docker.HostMount{
			Type:   mount.Type,
			Source: mount.Source,
			Target: mount.Destination,
		}
	}
	return mounts
}

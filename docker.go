package dexec

import (
	docker "github.com/fsouza/go-dockerclient"
)

// Docker contains connection to Docker API.
// Use github.com/fsouza/go-dockerclient to initialize *docker.Client.
type Docker struct {
	*docker.Client
}

// Command returns the Cmd struct to execute the named program with given
// arguments using specified execution method.
//
// For each new Cmd, you should create a new instance for "method" argument.
func (d Docker) Command(method Execution[Docker], name string, arg ...string) *DockerCmd {
	return &DockerCmd{
		GenericCmd: &GenericCmd[Docker]{
			Path:   name,
			Args:   arg,
			Method: method,
			client: d,
		},
	}
}

// DockerCmd represents an external command being prepared or run.
//
// A DockerCmd cannot be reused after calling its Run, Output or CombinedOutput
// methods.
type DockerCmd struct {
	*GenericCmd[Docker]
}

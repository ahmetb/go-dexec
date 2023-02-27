package dexec

import "io"

type ContainerClient interface {
	Docker | Containerd
}

// Execution determines how the command is going to be executed. Currently
// the only method is ByCreatingContainer.
type Execution[T ContainerClient] interface {
	create(d T, cmd []string) error
	run(d T, stdin io.Reader, stdout, stderr io.Writer) error
	wait(d T) (int, error)

	setEnv(env []string) error
	setDir(dir string) error
	getID() string
	kill(d T) error
	cleanup(d T) error
}

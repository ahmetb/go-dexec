package main

import (
	"fmt"

	"github.com/OneCloudInc/go-dexec"
	"github.com/fsouza/go-dockerclient"
)

func main() {
	cl, _ := docker.NewClientFromEnv()
	d := dexec.Docker{cl}
	m, _ := dexec.ByCreatingContainer(docker.CreateContainerOptions{
		Config: &docker.Config{Image: "busybox"}})

	cmd := d.Command(m, "sh", "-c", "exit 255;")
	err := cmd.Run()
	if err == nil {
		panic("not expecting successful exit")
	}

	if ee, ok := err.(*dexec.ExitError); ok {
		fmt.Printf("exit code=%d\n", ee.ExitCode) // <--
	} else {
		panic(err)
	}
}

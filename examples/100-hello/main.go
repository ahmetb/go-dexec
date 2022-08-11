package main

import (
	"fmt"

	"github.com/Workiva/go-dexec"
	"github.com/fsouza/go-dockerclient"
)

func main() {
	cl, _ := docker.NewClientFromEnv()
	d := dexec.Docker{cl}

	m, _ := dexec.ByCreatingContainer(docker.CreateContainerOptions{
		Config: &docker.Config{Image: "busybox"}})
	cmd := d.Command(m, "echo", `I am running inside a container!`)
	b, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", b)
}

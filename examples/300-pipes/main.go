package main

import (
	"fmt"
	"os"

	"github.com/Workiva/go-dexec"
	"github.com/fsouza/go-dockerclient"
)

func main() {
	cl, _ := docker.NewClientFromEnv()
	d := dexec.Docker{cl}

	m, _ := dexec.ByCreatingContainer(docker.CreateContainerOptions{
		Config: &docker.Config{Image: "busybox"}})

	cmd := d.Command(m, "tr", "[:lower:]", "[:upper:]")
	w, err := cmd.StdinPipe() // <--
	if err != nil {
		panic(err)
	}
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	fmt.Fprintln(w, "Hello world") // <--
	fmt.Fprintln(w, "from")        // <--
	fmt.Fprintln(w, "container")   // <--
	w.Close()

	if err := cmd.Wait(); err != nil {
		panic(err)
	}
	// Output:
	//   HELLO WORLD
	//   FROM
	//   CONTAINER
}

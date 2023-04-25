package dexec_test

func ExampleCmd_Output() {
	// AA: disabling docker tests for skynet
	/*	cl, _ := docker.NewClient("unix:///var/run/docker.sock")
			d := dexec.Docker{cl}

			m, _ := dexec.ByCreatingContainer(docker.CreateContainerOptions{
				Config: &docker.Config{Image: "busybox"}})

			cmd := d.Command(m, "echo", `I am running inside a container!`)
			b, err := cmd.Output()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%s", b)

		// Output: I am running inside a container!
	*/
}

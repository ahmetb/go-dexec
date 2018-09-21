package main

import (
	"fmt"
	"github.com/OneCloudInc/go-dexec"
	"github.com/fsouza/go-dockerclient"
	"io/ioutil"
)

const (
	url = `https://www.youtube.com/watch?v=2_79sx6V3tU` // don’t judge.
	out = "music.mp3"
)

func main() {
	cl, _ := docker.NewClientFromEnv()
	d := dexec.Docker{cl}

	m, _ := dexec.ByCreatingContainer(docker.CreateContainerOptions{
		Config: &docker.Config{Image: "vimagick/youtube-dl"}})
	cmd := d.Command(m, "sh", "-c", fmt.Sprintf("youtube-dl %s -o - | ffmpeg -i pipe:0 -f mp3 pipe:1", url))
	b, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(out, b, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Saved %d bytes to %s\n", len(b), out)
}

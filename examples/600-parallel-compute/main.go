package main

import (
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/OneCloudInc/go-dexec"
	"github.com/fsouza/go-dockerclient"
)

var (
	d    dexec.Docker
	sums = make(map[int]string)
)

func init() {
	// make sure your env is set to Swarm Manager.
	cl, err := docker.NewClientFromEnv()
	if err != nil {
		panic(err)
	}
	d = dexec.Docker{cl}
}

func main() {
	b, err := ioutil.ReadFile("files.txt")
	if err != nil {
		panic(err)
	}
	urls := strings.Split(strings.TrimSpace(string(b)), "\n")

	var wg sync.WaitGroup
	var m sync.Mutex
	for i, url := range urls {
		wg.Add(1)
		go func(n int, u string) { // Parallelize
			defer wg.Done()
			sum := md5(u)
			m.Lock()
			sums[n] = sum
			m.Unlock()
		}(i, url)
	}
	wg.Wait()

	for k, v := range sums {
		fmt.Printf("%d %s\n", k, v)
	}
}

func md5(url string) string {
	m, _ := dexec.ByCreatingContainer(docker.CreateContainerOptions{
		Config: &docker.Config{Image: "busybox"}})
	cmd := d.Command(m, "sh", "-c", fmt.Sprintf("wget -qO- %s | md5sum", url))

	b, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}
	return strings.TrimSpace(string(b))
}

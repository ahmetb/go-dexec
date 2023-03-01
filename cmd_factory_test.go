package dexec

import (
	"github.com/containerd/containerd"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"testing"
)

type fakeClient struct {
}

func getMounts() []Mount {
	return []Mount{
		{
			Type:        "bind",
			Source:      "/local/path",
			Destination: "/go/src",
			Options:     []string{"bind"},
		},
	}
}
func Test_convertMounts_Docker(t *testing.T) {
	actual := convertMounts[docker.HostMount](getMounts())
	assert.Len(t, actual, 1)
	expected := docker.HostMount{Type: "bind", Source: "/local/path", Target: "/go/src"}
	assert.Equal(t, expected, actual[0])
}

func Test_convertMounts_Containerd(t *testing.T) {
	actual := convertMounts[specs.Mount](getMounts())
	assert.Len(t, actual, 1)
	expected := specs.Mount{Type: "bind", Source: "/local/path", Destination: "/go/src", Options: []string{"bind"}}
	assert.Equal(t, expected, actual[0])
}

func TestCommand(t *testing.T) {
	cmd := Command(&docker.Client{}, Config{})
	assert.IsType(t, &DockerCmd{}, cmd)

	cmd = Command(&containerd.Client{}, Config{})
	assert.IsType(t, &ContainerdCmd{}, cmd)

	assert.Panics(t, func() {
		Command(&fakeClient{}, Config{})
	})
}

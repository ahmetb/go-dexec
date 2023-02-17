package dexec

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"testing"
)

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

func Test_convertMounts_ContainerD(t *testing.T) {
	actual := convertMounts[specs.Mount](getMounts())
	assert.Len(t, actual, 1)
	expected := specs.Mount{Type: "bind", Source: "/local/path", Destination: "/go/src", Options: []string{"bind"}}
	assert.Equal(t, expected, actual[0])
}

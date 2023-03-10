package dexec

import (
	"github.com/containerd/containerd"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
	"unsafe"
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

	// AA: this is dirty, but this is the only way we can set the
	// default namespace on the client without having a working
	// containerd socket to connect to
	cdClient := &containerd.Client{}
	pointerVal := reflect.ValueOf(cdClient)
	val := reflect.Indirect(pointerVal)
	defaultns := val.FieldByName("defaultns")
	ptrToDefaultNs := unsafe.Pointer(defaultns.UnsafeAddr())
	realPtrToDefaultNs := (*string)(ptrToDefaultNs)
	*realPtrToDefaultNs = "unit-test"

	cmd = Command(cdClient, Config{})
	assert.IsType(t, &ContainerdCmd{}, cmd)

	assert.Panics(t, func() {
		Command(&containerd.Client{}, Config{})
	})

	assert.Panics(t, func() {
		Command(&fakeClient{}, Config{})
	})
}

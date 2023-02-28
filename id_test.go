package dexec

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

func TestRandomString(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{
			name: "length 6",
			n:    6,
		},
		{
			name: "length 12",
			n:    12,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := RandomString(tt.n)
			assert.Len(t, actual, tt.n)
			regex := fmt.Sprintf("[A-Za-z]{%d}", tt.n)
			assert.Regexp(t, regexp.MustCompile(regex), actual)
		})
	}
}

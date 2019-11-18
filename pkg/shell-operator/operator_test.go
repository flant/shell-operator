package shell_operator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	curdir, err := os.Getwd()
	assert.NoError(t, err)

	tests := []struct {
		name    string
		path    string
		absPath string
	}{
		{
			"empty is cwd",
			"",
			"CWD",
		},
		{
			"dot is cwd",
			".",
			"CWD",
		},
		{
			"dir in curdir",
			"testdata/crd",
			"CWD/testdata/crd",
		},
		{
			"curdir as dot",
			"./testdata/crd",
			"CWD/testdata/crd",
		},
		{
			"two dots",
			"../testdata/crd",
			"CWD/../testdata/crd",
		},
		{
			"abs path",
			"/testdata",
			"/testdata",
		},
		{
			"two dots in path",
			"testdata/../crd",
			"CWD/crd",
		},
		{
			"two dots in path with a dot",
			"./testdata/../crd",
			"CWD/crd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := filepath.Abs(tt.path)
			assert.NoError(t, err)
			expected := strings.Replace(tt.absPath, "CWD", curdir, 1)
			expected = filepath.Clean(expected)
			assert.Equal(t, expected, res)
			fmt.Printf("IN:  %s\nOUT: %s\n", tt.path, res)
		})
	}
}

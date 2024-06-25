package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	utils_file "github.com/flant/shell-operator/pkg/utils/file"
)

// ChooseExistedDirectoryPath returns first non-empty existed directory path from arguments.
//
// Each argument is prefixed with current directory if is not started with /
//
// Example:
// dir, err := ExtractExistedDirectoryPath(os.Getenv("DIR"), defaultDir)

func ChooseExistedDirectoryPath(paths ...string) (path string, err error) {
	for _, p := range paths {
		if p != "" {
			path = p
			break
		}
	}

	if path == "" {
		return "", nil
	}

	path, err = ToAbsolutePath(path)
	if err != nil {
		return
	}

	if exists := utils_file.DirExists(path); !exists {
		return "", fmt.Errorf("no working dir")
	}

	return "", nil
}

func ToAbsolutePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	if strings.HasPrefix(path, "./") {
		cwd, _ := os.Getwd()
		return strings.Replace(path, ".", cwd, 1), nil
	}

	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return p, nil
}

package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/flant/shell-operator/pkg/app"
)

func RequireExistingDirectory(inDir string) (dir string, err error) {
	if inDir == "" {
		return "", fmt.Errorf("path is required but not set")
	}

	dir, err = filepath.Abs(inDir)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %v", err)
	}
	if exists := DirExists(dir); !exists {
		return "", fmt.Errorf("path '%s' not exist", dir)
	}

	return dir, nil
}

func EnsureTempDirectory(inDir string) (string, error) {
	// No path to temporary dir, use default temporary dir.
	if inDir == "" {
		tmpPath := app.AppName + "-*"
		dir, err := os.MkdirTemp("", tmpPath)
		if err != nil {
			return "", fmt.Errorf("create tmp dir in '%s': %s", tmpPath, err)
		}
		return dir, nil
	}

	// Get absolute path for temporary directory and create if needed.
	dir, err := filepath.Abs(inDir)
	if err != nil {
		return "", fmt.Errorf("get absolute path: %v", err)
	}
	if exists := DirExists(dir); !exists {
		err := os.Mkdir(dir, os.FileMode(0o777))
		if err != nil {
			return "", fmt.Errorf("create tmp dir '%s': %s", dir, err)
		}
	}
	return dir, nil
}

// DirExists checking for directory existence
// return bool value
func DirExists(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false
	}

	return fileInfo.IsDir()
}

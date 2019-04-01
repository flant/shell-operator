package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// FileExists returns true if path exists
func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func DirExists(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false, nil
	}

	return fileInfo.IsDir(), nil
}

func IsFileExecutable(f os.FileInfo) bool {
	return f.Mode()&0111 != 0
}


// RecursiveGetExecutablePaths finds recursively all executable files
// inside a dir directory. Hidden directories and files are ignored.
func RecursiveGetExecutablePaths(dir string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if f.IsDir() {
			// Skip hidden directories inside initial directory
			if strings.HasPrefix(f.Name(), ".") {
				return filepath.SkipDir
			}

			return nil
		}

		// ignore hidden files
		if strings.HasPrefix(f.Name(), ".") {
			return nil
		}

		if IsFileExecutable(f) {
			paths = append(paths, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return paths, nil
}
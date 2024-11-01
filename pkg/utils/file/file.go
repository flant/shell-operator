package utils

import (
	"os"
	"path/filepath"
	"strings"

	log "github.com/deckhouse/deckhouse/go_lib/log"
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

func IsFileExecutable(f os.FileInfo) bool {
	return f.Mode()&0o111 != 0
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
			// Skip hidden and lib directories inside initial directory
			if strings.HasPrefix(f.Name(), ".") || f.Name() == "lib" {
				return filepath.SkipDir
			}

			return nil
		}

		if !isExecutableHookFile(f) {
			log.Warnf("File '%s' is skipped: no executable permissions, chmod +x is required to run this hook", path)
			return nil
		}

		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}

// RecursiveCheckLibDirectory finds recursively all executable files
// inside a lib directory. And will log warning with these files.
func RecursiveCheckLibDirectory(dir string) error {
	dir = filepath.Join(dir, "lib")
	if exist := DirExists(dir); !exist {
		return nil
	}

	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if f.IsDir() {
			// Skip hidden directory inside initial directory
			if strings.HasPrefix(f.Name(), ".") {
				return filepath.SkipDir
			}

			return nil
		}
		if isExecutableHookFile(f) {
			log.Warnf("File '%s' has executable permissions and is located in the ignored 'lib' directory", strings.TrimPrefix(path, dir))
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func isExecutableHookFile(f os.FileInfo) bool {
	// ignore hidden files
	if strings.HasPrefix(f.Name(), ".") {
		return false
	}

	// ignore .yaml, .json, .txt, .md files
	switch filepath.Ext(f.Name()) {
	case ".yaml", ".json", ".md", ".txt":
		return false
	}

	return IsFileExecutable(f)
}

package utils

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"
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

func CheckExecutablePermissions(f os.FileInfo) error {
	if f.Mode()&0o111 == 0 {
		return ErrFileNoExecutablePermissions
	}

	return nil
}

// RecursiveGetExecutablePaths finds recursively all executable files
// inside a dir directory. Hidden directories and files are ignored.
func RecursiveGetExecutablePaths(dir string, excludedDirs ...string) ([]string, error) {
	paths := make([]string, 0)
	excludedDirs = append(excludedDirs, "lib")
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if f.IsDir() {
			// Skip hidden and lib directories inside initial directory
			if strings.HasPrefix(f.Name(), ".") || slices.Contains(excludedDirs, f.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		if err := checkExecutableHookFile(f); err != nil {
			if errors.Is(err, ErrFileNoExecutablePermissions) {
				log.Warn("file is skipped", slog.String("path", path), log.Err(err))

				return nil
			}

			log.Debug("file is skipped", slog.String("path", path), log.Err(err))

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

		if err := checkExecutableHookFile(f); err == nil {
			log.Warn("file has executable permissions and is located in the ignored 'lib' directory",
				slog.String("file", strings.TrimPrefix(path, dir)))
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

var (
	ErrFileHasWrongExtension       = errors.New("file has wrong extension")
	ErrFileIsHidden                = errors.New("file is hidden")
	ErrFileNoExecutablePermissions = errors.New("no executable permissions, chmod +x is required to run this hook")
)

func checkExecutableHookFile(f os.FileInfo) error {
	// ignore hidden files
	if strings.HasPrefix(f.Name(), ".") {
		return ErrFileIsHidden
	}

	// ignore .yaml, .json, .txt, .md files
	switch filepath.Ext(f.Name()) {
	case ".yaml", ".json", ".md", ".txt":
		return ErrFileHasWrongExtension
	}

	return CheckExecutablePermissions(f)
}

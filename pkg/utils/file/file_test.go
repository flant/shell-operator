package utils

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

func prepareTestDirTree() (string, error) {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %v\n", err)
	}

	if err = os.MkdirAll(filepath.Join(tmpDir, "aa"), 0o755); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	if err = os.MkdirAll(filepath.Join(tmpDir, "lib"), 0o755); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	if err = createExecutableFile(filepath.Join(tmpDir, "aa/exec.py")); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	if err = createExecutableFile(filepath.Join(tmpDir, "check.py")); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	if err = createExecutableFile(filepath.Join(filepath.Join(tmpDir, "lib"), "lib.py")); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return tmpDir, nil
}

func createExecutableFile(file string) error {
	if _, err := os.Create(file); err != nil {
		return err
	}
	os.Chmod(file, 0o777)

	return nil
}

func TestRecursiveGetExecutablePaths(t *testing.T) {
	dir, err := prepareTestDirTree()
	if err != nil {
		t.Fatalf("error creating temp directory: %v\n", err)
	}
	type args struct {
		dir string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "get executable files",
			args: args{
				dir: dir,
			},
			want:    []string{"aa/exec.py", "check.py"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RecursiveGetExecutablePaths(tt.args.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecursiveGetExecutablePaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for i := range got {
				if !strings.HasSuffix(got[i], tt.want[i]) {
					t.Errorf("RecursiveGetExecutablePaths() got = %v, want %v", got, tt.want)
				}
			}
		})
	}

	os.RemoveAll(dir)
}

func TestRecursiveCheckLibDirectory(t *testing.T) {
	dir, err := prepareTestDirTree()
	if err != nil {
		t.Fatalf("error creating temp directory: %v\n", err)
	}

	type args struct {
		dir string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "check lib directory",
			args: args{
				dir: dir,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		var buf bytes.Buffer
		log.SetOutput(&buf)

		formatter := new(log.TextFormatter)
		formatter.DisableColors = true
		formatter.DisableTimestamp = true
		log.SetFormatter(formatter)

		t.Run(tt.name, func(t *testing.T) {
			if err := RecursiveCheckLibDirectory(tt.args.dir); (err != nil) != tt.wantErr {
				t.Errorf("RecursiveCheckLibDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}

			if strings.Compare(
				strings.TrimSpace(buf.String()),
				`level=warning msg="File '/lib.py' has executable permissions and is located in the ignored 'lib' directory"`) != 0 {
				t.Errorf("RecursiveCheckLibDirectory() error, got `%v`", buf.String())
			}
		})
	}

	os.RemoveAll(dir)
}

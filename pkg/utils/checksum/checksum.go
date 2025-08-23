package checksum

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"

	"github.com/cespare/xxhash/v2"
)

func CalculateChecksum(stringArr ...string) string {
	digest := xxhash.New()
	sort.Strings(stringArr)
	for _, value := range stringArr {
		_, _ = digest.Write([]byte(value))
	}

	return hex.EncodeToString(digest.Sum(nil))
}

func CalculateChecksumOfFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return CalculateChecksum(string(content)), nil
}

func CalculateChecksumOfDirectory(path string) (string, error) {
	res := ""

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		checksum, err := CalculateChecksumOfFile(path)
		if err != nil {
			return err
		}
		res = CalculateChecksum(res, checksum)

		return nil
	})
	if err != nil {
		return "", err
	}

	return res, nil
}

func CalculateChecksumOfPaths(pathArr ...string) (string, error) {
	res := ""

	for _, path := range pathArr {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return "", err
		}

		var checksum string
		if fileInfo.IsDir() {
			checksum, err = CalculateChecksumOfDirectory(path)
		} else {
			checksum, err = CalculateChecksumOfFile(path)
		}

		if err != nil {
			return "", err
		}
		res = CalculateChecksum(res, checksum)
	}

	return res, nil
}

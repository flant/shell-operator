package checksum

import (
	"crypto/md5"
	"encoding/hex"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

var md5Pool = sync.Pool{
	New: func() any { return md5.New() },
}

func CalculateChecksum(stringArr ...string) string {
	h := md5Pool.Get().(hash.Hash)
	h.Reset()
	sort.Strings(stringArr)
	for _, value := range stringArr {
		_, _ = h.Write([]byte(value))
	}
	sum := hex.EncodeToString(h.Sum(nil))
	md5Pool.Put(h)
	return sum
}

// CalculateChecksumOfBytes computes an MD5 hex digest directly from a byte
// slice, avoiding the []byte→string→[]byte round-trip that CalculateChecksum
// would require.
func CalculateChecksumOfBytes(data []byte) string {
	h := md5Pool.Get().(hash.Hash)
	h.Reset()
	_, _ = h.Write(data)
	sum := hex.EncodeToString(h.Sum(nil))
	md5Pool.Put(h)
	return sum
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

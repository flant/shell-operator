package checksum

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"

	"github.com/cespare/xxhash/v2"
)

// CalculateChecksum вычисляет 64-битную контрольную сумму для массива строк.
func CalculateChecksum(stringArr ...string) uint64 {
	digest := xxhash.New()
	sort.Strings(stringArr)
	for _, value := range stringArr {
		_, _ = digest.Write([]byte(value))
	}
	return digest.Sum64()
}

func CalculateChecksumOfFile(path string) (uint64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return CalculateChecksum(string(content)), nil
}

func combineHashes(h1, h2 uint64) uint64 {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf[:8], h1)
	binary.BigEndian.PutUint64(buf[8:], h2)
	return xxhash.Sum64(buf)
}

func CalculateChecksumOfDirectory(path string) (uint64, error) {
	var res uint64

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
		res = combineHashes(res, checksum)

		return nil
	})
	if err != nil {
		return 0, err
	}

	return res, nil
}

func CalculateChecksumOfPaths(pathArr ...string) (uint64, error) {
	var res uint64

	for _, path := range pathArr {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return 0, err
		}

		var checksum uint64
		if fileInfo.IsDir() {
			checksum, err = CalculateChecksumOfDirectory(path)
		} else {
			checksum, err = CalculateChecksumOfFile(path)
		}

		if err != nil {
			return 0, err
		}
		res = combineHashes(res, checksum)
	}

	return res, nil
}

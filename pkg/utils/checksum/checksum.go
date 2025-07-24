package checksum

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/cespare/xxhash/v2"
)

// CalculateChecksum calculates a fast and stable checksum for any data structure.
// optimized with pools for both encoders and hashers.
func CalculateChecksum(v interface{}) (uint64, error) {
	switch v := v.(type) {
	case nil:
		return 0, nil
	case string:
		hasher := getHasher()
		_, _ = hasher.WriteString(v)
		return hasher.Sum64(), nil
	case []byte:
		hasher := getHasher()
		_, _ = hasher.Write(v)
		return hasher.Sum64(), nil
	}

	mainHasher := getHasher()
	defer putHasher(mainHasher)
	// hasher's state must be reset after retrieving it from the pool
	mainHasher.Reset()

	encoder := getEncoder(mainHasher)
	defer putEncoder(encoder)

	if err := encoder.encode(reflect.ValueOf(v)); err != nil {
		return 0, err
	}

	return mainHasher.Sum64(), nil
}

// encoder is the state of our recursive "hashing serializer".
// for backward compatibility
func CalculateChecksum_old(stringArr ...string) string {
	hasher := md5.New()
	sort.Strings(stringArr)
	for _, value := range stringArr {
		_, _ = hasher.Write([]byte(value))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// CalculateChecksum_v1 calculates a 64-bit checksum for an array of strings. Renamed to avoid conflicts.
func CalculateChecksum_v1(stringArr ...string) uint64 {
	digest := xxhash.New()
	// Strings are sorted to ensure the checksum is stable.
	sortedStrings := make([]string, len(stringArr))
	copy(sortedStrings, stringArr)
	sort.Strings(sortedStrings)
	for _, value := range sortedStrings {
		_, _ = digest.Write([]byte(value))
	}
	return digest.Sum64()
}

func CalculateChecksumOfFile(path string) (uint64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return CalculateChecksum_v1(string(content)), nil
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

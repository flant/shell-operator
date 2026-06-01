package checksum

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

var md5Pool = sync.Pool{
	New: func() any { return md5.New() },
}

// CalculateChecksumOfObject computes a deterministic MD5 hex digest of an
// arbitrary value without serializing it to JSON. It is intended for the
// informers flow, where objects are already decomposed `map[string]interface{}`
// trees (unstructured content) and re-marshalling them to JSON purely to
// checksum them is wasteful. Map keys are visited in sorted order so the
// digest is stable across runs, and every scalar/composite is length-prefixed
// and type-tagged so structurally different values cannot collide.
func CalculateChecksumOfObject(v interface{}) string {
	h := md5Pool.Get().(hash.Hash)
	h.Reset()
	writeValueChecksum(h, v)
	sum := hex.EncodeToString(h.Sum(nil))
	md5Pool.Put(h)
	return sum
}

// type tags keep structurally different values from colliding in the digest.
const (
	tagNil    = 0x00
	tagBool   = 0x01
	tagString = 0x02
	tagInt    = 0x03
	tagFloat  = 0x04
	tagMap    = 0x05
	tagSlice  = 0x06
	tagOther  = 0x07
)

func writeValueChecksum(h hash.Hash, v interface{}) {
	switch t := v.(type) {
	case nil:
		_, _ = h.Write([]byte{tagNil})
	case bool:
		b := byte(0)
		if t {
			b = 1
		}
		_, _ = h.Write([]byte{tagBool, b})
	case string:
		_, _ = h.Write([]byte{tagString})
		writeLenString(h, t)
	case int64:
		_, _ = h.Write([]byte{tagInt})
		writeLenString(h, strconv.FormatInt(t, 10))
	case int:
		writeValueChecksum(h, int64(t))
	case int32:
		writeValueChecksum(h, int64(t))
	case uint:
		writeValueChecksum(h, int64(t))
	case uint32:
		writeValueChecksum(h, int64(t))
	case uint64:
		writeValueChecksum(h, int64(t))
	case float64:
		_, _ = h.Write([]byte{tagFloat})
		writeLenString(h, strconv.FormatFloat(t, 'g', -1, 64))
	case float32:
		writeValueChecksum(h, float64(t))
	case map[string]interface{}:
		_, _ = h.Write([]byte{tagMap})
		writeUint(h, uint64(len(t)))
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			writeLenString(h, k)
			writeValueChecksum(h, t[k])
		}
	case []interface{}:
		_, _ = h.Write([]byte{tagSlice})
		writeUint(h, uint64(len(t)))
		for _, item := range t {
			writeValueChecksum(h, item)
		}
	default:
		// Fall back to a stable textual rendering for values that are not
		// part of the standard unstructured content (e.g. Go-hook filter
		// results that return typed structs). This stays deterministic
		// without round-tripping through encoding/json.
		_, _ = h.Write([]byte{tagOther})
		writeLenString(h, fmt.Sprintf("%+v", t))
	}
}

func writeLenString(h hash.Hash, s string) {
	writeUint(h, uint64(len(s)))
	_, _ = h.Write([]byte(s))
}

func writeUint(h hash.Hash, n uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], n)
	_, _ = h.Write(buf[:])
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

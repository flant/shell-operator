package checksum

import (
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/cespare/xxhash/v2"
)

// avoid allocating and freeing memory for each object.
var encoderPool = sync.Pool{
	New: func() interface{} {
		return &encoder{}
	},
}

var hasherPool = sync.Pool{
	New: func() interface{} {
		return xxhash.New()
	},
}

func getEncoder(h io.Writer) *encoder {
	e := encoderPool.Get().(*encoder)
	e.h = h
	return e
}

func putEncoder(e *encoder) {
	encoderPool.Put(e)
}

func getHasher() *xxhash.Digest {
	return hasherPool.Get().(*xxhash.Digest)
}

func putHasher(h *xxhash.Digest) {
	hasherPool.Put(h)
}

type encoder struct {
	h io.Writer
}

// encode is the main recursive function.
func (e *encoder) encode(v reflect.Value) error {
	visited := make(map[uintptr]bool)
	return e.encodeWithVisited(v, visited)
}

func (e *encoder) encodeWithVisited(v reflect.Value, visited map[uintptr]bool) error {
	if !v.IsValid() {
		return nil
	}

	// Check for cycles on reference types first.
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice:
		if v.IsNil() {
			return nil
		}
		ptr := v.Pointer()
		if visited[ptr] {
			return nil // Cycle detected.
		}
		visited[ptr] = true
	}

	// Now proceed with the actual encoding based on kind.
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return e.encode(v.Elem())

	case reflect.Bool:
		return binary.Write(e.h, binary.BigEndian, v.Bool())

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return binary.Write(e.h, binary.BigEndian, v.Int())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return binary.Write(e.h, binary.BigEndian, v.Uint())

	case reflect.Float32, reflect.Float64:
		return binary.Write(e.h, binary.BigEndian, v.Float())

	case reflect.String:
		_, err := io.WriteString(e.h, v.String())
		return err

	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := e.encode(v.Index(i)); err != nil {
				return err
			}
		}

	case reflect.Map:
		var totalHash uint64
		// Use MapRange to avoid allocating a slice for keys, which is a significant optimization.
		iter := v.MapRange()

		// Get a pooled encoder for key-value pair hashing.
		pairHasher := getHasher()
		defer putHasher(pairHasher)
		pairEncoder := getEncoder(pairHasher)
		defer putEncoder(pairEncoder)

		for iter.Next() {
			key := iter.Key()
			val := iter.Value()

			// The hasher within pairEncoder is a new instance, but the visited map is shared.
			pairHasher.Reset()

			if err := pairEncoder.encodeWithVisited(key, visited); err != nil {
				return err
			}
			if err := pairEncoder.encodeWithVisited(val, visited); err != nil {
				return err
			}
			totalHash ^= pairHasher.Sum64()
		}
		return binary.Write(e.h, binary.BigEndian, totalHash)

	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if t.Field(i).IsExported() {
				if _, err := io.WriteString(e.h, t.Field(i).Name); err != nil {
					return err
				}
				if err := e.encodeWithVisited(v.Field(i), visited); err != nil {
					return err
				}
			}
		}

	default:
		return fmt.Errorf("unsupported type for checksum calculation: %s", v.Kind())
	}

	return nil
}

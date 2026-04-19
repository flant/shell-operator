package checksum

import (
	"sync"
	"testing"
)

func TestCalculateChecksum_OrderIndependent(t *testing.T) {
	chksum1 := CalculateChecksum("host_1", "host_2")
	chksum2 := CalculateChecksum("host_2", "host_1")

	if chksum1 != chksum2 {
		t.Errorf("checksums should be identical for identical content regardless of order: %s vs %s", chksum1, chksum2)
	}
}

func TestCalculateChecksum_DifferentInputs(t *testing.T) {
	chksum1 := CalculateChecksum("aaa")
	chksum2 := CalculateChecksum("bbb")

	if chksum1 == chksum2 {
		t.Errorf("checksums should differ for different content: both are %s", chksum1)
	}
}

func TestCalculateChecksum_Empty(t *testing.T) {
	chksum := CalculateChecksum()
	if chksum == "" {
		t.Error("checksum of empty input should not be empty string")
	}
}

func TestCalculateChecksum_Single(t *testing.T) {
	chksum := CalculateChecksum("hello")
	if chksum == "" {
		t.Error("checksum should not be empty")
	}
	if len(chksum) != 32 {
		t.Errorf("md5 hex digest should be 32 chars, got %d", len(chksum))
	}
}

func TestCalculateChecksumOfBytes(t *testing.T) {
	data := []byte(`{"foo":"bar"}`)
	chksum := CalculateChecksumOfBytes(data)

	if chksum == "" {
		t.Error("checksum should not be empty")
	}
	if len(chksum) != 32 {
		t.Errorf("md5 hex digest should be 32 chars, got %d", len(chksum))
	}
}

func TestCalculateChecksumOfBytes_MatchesStringVersion(t *testing.T) {
	input := `{"key":"value","num":42}`
	fromString := CalculateChecksum(input)
	fromBytes := CalculateChecksumOfBytes([]byte(input))

	if fromString != fromBytes {
		t.Errorf("CalculateChecksumOfBytes should match CalculateChecksum for single string: %s vs %s", fromString, fromBytes)
	}
}

func TestCalculateChecksumOfBytes_DifferentInputs(t *testing.T) {
	chksum1 := CalculateChecksumOfBytes([]byte("aaa"))
	chksum2 := CalculateChecksumOfBytes([]byte("bbb"))

	if chksum1 == chksum2 {
		t.Errorf("checksums should differ for different content: both are %s", chksum1)
	}
}

func TestCalculateChecksumOfBytes_Empty(t *testing.T) {
	chksum := CalculateChecksumOfBytes([]byte{})
	if chksum == "" {
		t.Error("checksum of empty input should not be empty string")
	}
}

func TestCalculateChecksum_ConcurrentSafety(t *testing.T) {
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make([]string, goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			results[idx] = CalculateChecksum("concurrent-test")
		}(i)
	}
	wg.Wait()

	for i := 1; i < goroutines; i++ {
		if results[i] != results[0] {
			t.Errorf("concurrent results should be identical: results[0]=%s, results[%d]=%s", results[0], i, results[i])
		}
	}
}

func TestCalculateChecksumOfBytes_ConcurrentSafety(t *testing.T) {
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	data := []byte("concurrent-test-bytes")
	results := make([]string, goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			results[idx] = CalculateChecksumOfBytes(data)
		}(i)
	}
	wg.Wait()

	for i := 1; i < goroutines; i++ {
		if results[i] != results[0] {
			t.Errorf("concurrent results should be identical: results[0]=%s, results[%d]=%s", results[0], i, results[i])
		}
	}
}

func BenchmarkCalculateChecksum(b *testing.B) {
	input := `{"metadata":{"name":"my-pod","namespace":"default","labels":{"app":"test"}}}`
	b.ResetTimer()
	for range b.N {
		CalculateChecksum(input)
	}
}

func BenchmarkCalculateChecksumOfBytes(b *testing.B) {
	input := []byte(`{"metadata":{"name":"my-pod","namespace":"default","labels":{"app":"test"}}}`)
	b.ResetTimer()
	for range b.N {
		CalculateChecksumOfBytes(input)
	}
}

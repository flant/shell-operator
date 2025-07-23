package checksum

import (
	"fmt"
	"testing"
)

func TestChecksum(t *testing.T) {
	val1 := []string{
		"host_1",
		"host_2",
	}
	chksum1 := CalculateChecksum(val1...)
	fmt.Printf("val1 chksum: %d\n", chksum1)

	val2 := []string{
		"host_2",
		"host_1",
	}
	chksum2 := CalculateChecksum(val2...)
	fmt.Printf("val2 chksum: %d\n", chksum2)

	if chksum1 != chksum2 {
		t.Errorf("checksums not identical for identical content")
	}
}

func TestCalculateChecksum(t *testing.T) {
	val1 := []string{"val1", "val2", "val3"}
	val2 := []string{"val3", "val2", "val1"}

	chksum1 := CalculateChecksum(val1...)
	if chksum1 == 0 {
		t.Errorf("Checksum is 0")
	}

	chksum2 := CalculateChecksum(val2...)
	if chksum1 != chksum2 {
		t.Errorf("Checksums are not equal for %v and %v: %d != %d", val1, val2, chksum1, chksum2)
	}
}

var (
	// Пример данных, имитирующих JSON объекта Kubernetes
	testData = `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-pod","namespace":"default","resourceVersion":"12345","uid":"a1b2c3d4-e5f6-7890-1234-567890abcdef"},"spec":{"containers":[{"image":"nginx","name":"nginx"}]}}`
)

func BenchmarkCalculateChecksum_xxHash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateChecksum(testData)
	}
}

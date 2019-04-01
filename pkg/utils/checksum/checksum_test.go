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
	fmt.Printf("val1 chksum: %s\n", chksum1)

	val2 := []string{
		"host_2",
		"host_1",
	}
	chksum2 := CalculateChecksum(val2...)
	fmt.Printf("val2 chksum: %s\n", chksum2)

	if chksum1 != chksum2 {
		t.Errorf("checksums not identical for identical content")
	}
}

package dump

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Sort_ByNamespaceAndName(t *testing.T) {
	sorted := []string{
		"main",
		"asd",
		"hook.kube",
		"sched-queue",
		"str",
	}

	assert.True(t, sort.IsSorted(AsQueueNames(sorted)))

	input := []string{
		"str",
		"asd",
		"hook.kube",
		"main",
		"sched-queue",
	}

	sort.Sort(AsQueueNames(input))

	for i, s := range sorted {
		assert.Equal(t, s, input[i], "input index %d", i)
	}
}

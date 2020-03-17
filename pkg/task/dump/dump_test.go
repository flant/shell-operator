package dump

import (
	"sort"
	"testing"

	. "github.com/onsi/gomega"
)

func Test_Sort_ByNamespaceAndName(t *testing.T) {
	g := NewWithT(t)
	sorted := []string{
		"main",
		"asd",
		"hook.kube",
		"sched-queue",
		"str",
	}

	g.Expect(sort.IsSorted(AsQueueNames(sorted))).To(BeTrue())

	input := []string{
		"str",
		"asd",
		"hook.kube",
		"main",
		"sched-queue",
	}

	sort.Sort(AsQueueNames(input))

	for i, s := range sorted {
		g.Expect(input[i]).Should(Equal(s), "fail on 'input' index %d", i)
	}
}

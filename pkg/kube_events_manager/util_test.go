package kubeeventsmanager

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_RandomizedResyncPeriod(t *testing.T) {
	t.SkipNow()
	for i := 0; i < 10; i++ {
		p := randomizedResyncPeriod()
		fmt.Printf("%02d. %s\n", i, p.String())
	}
}

func TestResourceIDStore_Basic(t *testing.T) {
	store := NewResourceIDStore()

	// Test Kinds
	podIdx := store.GetKindIDX("Pod")
	assert.Equal(t, uint16(0), podIdx)
	assert.Equal(t, "Pod", store.GetKindByID(podIdx))

	depIdx := store.GetKindIDX("Deployment")
	assert.Equal(t, uint16(1), depIdx)
	assert.Equal(t, "Deployment", store.GetKindByID(depIdx))

	// Test that repeated calls return the same index
	podIdx2 := store.GetKindIDX("Pod")
	assert.Equal(t, podIdx, podIdx2)

	// Test Namespaces
	nsDefaultIdx := store.GetNSIDX("default")
	assert.Equal(t, uint16(0), nsDefaultIdx)
	assert.Equal(t, "default", store.GetNSByID(nsDefaultIdx))

	nsKubeSysIdx := store.GetNSIDX("kube-system")
	assert.Equal(t, uint16(1), nsKubeSysIdx)
	assert.Equal(t, "kube-system", store.GetNSByID(nsKubeSysIdx))

	// Test that repeated calls return the same index
	nsDefaultIdx2 := store.GetNSIDX("default")
	assert.Equal(t, nsDefaultIdx, nsDefaultIdx2)

	// Test unknown index
	assert.Equal(t, "", store.GetKindByID(999))
	assert.Equal(t, "", store.GetNSByID(999))
}

func TestResourceIDStore_ThreadSafety(t *testing.T) {
	store := NewResourceIDStore()
	kinds := []string{"Pod", "Service", "Deployment", "ConfigMap", "Secret"}
	namespaces := []string{"default", "kube-system", "monitoring", "ingress-nginx", "cert-manager"}

	var wg sync.WaitGroup
	goroutineCount := 50
	iterations := 100

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Access in a semi-random pattern
				store.GetKindIDX(kinds[j%len(kinds)])
				store.GetNSIDX(namespaces[j%len(namespaces)])
			}
		}()
	}

	wg.Wait()

	// After all concurrent access, check for consistency
	assert.Equal(t, len(kinds), len(store.kindStore))
	assert.Equal(t, len(namespaces), len(store.nsStore))

	// Verify that all original kinds and namespaces are present and have consistent IDs
	for _, kind := range kinds {
		id := store.GetKindIDX(kind)
		retrievedKind := store.GetKindByID(id)
		assert.Equal(t, kind, retrievedKind)
	}
	for _, ns := range namespaces {
		id := store.GetNSIDX(ns)
		retrievedNs := store.GetNSByID(id)
		assert.Equal(t, ns, retrievedNs)
	}
}

// --- Benchmarks ---

var (
	benchmarkKinds      = []string{"Pod", "Service", "Deployment", "ConfigMap", "Secret", "ReplicaSet", "DaemonSet", "Job", "CronJob", "Ingress"}
	benchmarkNamespaces = []string{"default", "kube-system", "monitoring", "ingress-nginx", "cert-manager", "long-namespace-name-for-testing", "another-one-with-more-chars"}
	benchmarkName       = "my-resource-name-with-random-suffix-12345"
)

func Benchmark_OldResourceID(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		kind := benchmarkKinds[i%len(benchmarkKinds)]
		ns := benchmarkNamespaces[i%len(benchmarkNamespaces)]
		result := fmt.Sprintf("%s/%s/%s", ns, kind, benchmarkName)
		_ = result
	}
}

func Benchmark_NewResourceID(b *testing.B) {
	store := NewResourceIDStore()
	for _, kind := range benchmarkKinds {
		store.GetKindIDX(kind)
	}
	for _, ns := range benchmarkNamespaces {
		store.GetNSIDX(ns)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		kind := benchmarkKinds[i%len(benchmarkKinds)]
		ns := benchmarkNamespaces[i%len(benchmarkNamespaces)]

		result := ResourceID{
			NamespaceIDX: store.GetNSIDX(ns),
			KindIDX:      store.GetKindIDX(kind),
			Name:         benchmarkName,
		}
		_ = result
	}
}

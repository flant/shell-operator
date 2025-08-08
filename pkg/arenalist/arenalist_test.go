package arenalist

import (
	"container/list"
	"runtime"
	"testing"
)

func Benchmark_STDLib_LinkedList_PushBack(b *testing.B) {
	l := list.New()
	for i := 0; i < b.N; i++ {
		l.PushBack(i)
	}
}

func Benchmark_ArenaList_PushBack(b *testing.B) {
	l := New[int]()
	for i := 0; i < b.N; i++ {
		l.PushBack(i)
	}
}

// Benchmarks for a large number of elements
func Benchmark_STDLib_LinkedList_100kElements(b *testing.B) {
	for i := 0; i < b.N; i++ {
		l := list.New()
		for j := 0; j < 100000; j++ {
			l.PushBack(j)
		}
	}
}

func Benchmark_ArenaList_100kElements(b *testing.B) {
	for i := 0; i < b.N; i++ {
		l := New[int]()
		for j := 0; j < 100000; j++ {
			l.PushBack(j)
		}
	}
}

// Benchmarks for insert operations in the middle
func Benchmark_STDLib_LinkedList_InsertMiddle(b *testing.B) {
	l := list.New()
	// Prepare the list with elements
	for i := 0; i < 100000; i++ {
		l.PushBack(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Insert in the middle
		mid := l.Front()
		for j := 0; j < 50000; j++ {
			mid = mid.Next()
		}
		l.InsertBefore(i, mid)
	}
}

func Benchmark_ArenaList_InsertMiddle(b *testing.B) {
	l := New[int]()
	// Prepare the list with elements
	for i := 0; i < 100000; i++ {
		l.PushBack(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Insert in the middle
		mid := l.Front()
		for j := 0; j < 50000; j++ {
			mid = mid.Next()
		}
		l.InsertBefore(i, mid)
	}
}

// Benchmarks for iterating over the list
func Benchmark_STDLib_LinkedList_Iteration(b *testing.B) {
	l := list.New()
	for i := 0; i < 100000; i++ {
		l.PushBack(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sum := 0
		for e := l.Front(); e != nil; e = e.Next() {
			sum += e.Value.(int)
		}
		_ = sum
	}
}

func Benchmark_ArenaList_Iteration(b *testing.B) {
	l := New[int]()
	for i := 0; i < 100000; i++ {
		l.PushBack(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sum := 0
		for e := l.Front(); e != nil; e = e.Next() {
			sum += e.Value
		}
		_ = sum
	}
}

// Benchmarks for removing elements
func Benchmark_STDLib_LinkedList_Remove(b *testing.B) {
	l := list.New()
	for i := 0; i < 100000; i++ {
		l.PushBack(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Remove every second element
		e := l.Front()
		for e != nil {
			next := e.Next()
			if e.Value.(int)%2 == 0 {
				l.Remove(e)
			}
			e = next
		}
	}
}

func Benchmark_ArenaList_Remove(b *testing.B) {
	l := New[int]()
	for i := 0; i < 100000; i++ {
		l.PushBack(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Remove every second element
		e := l.Front()
		for e != nil {
			next := e.Next()
			if e.Value%2 == 0 {
				l.Remove(e)
			}
			e = next
		}
	}
}

// List integrity check: head/tail/next/prev/len
func checkListIntegrity[T comparable](t *testing.T, l *List[T]) {
	n := 0
	prev := (*Element[T])(nil)
	for e := l.Front(); e != nil; e = e.Next() {
		if e.prev != prev {
			t.Fatalf("Broken prev link at %v", e.Value)
		}
		prev = e
		n++
	}
	if n != l.Len() {
		t.Fatalf("List.Len()=%d, but counted %d", l.Len(), n)
	}
	if (l.head == nil) != (l.tail == nil) {
		t.Fatalf("Head/tail mismatch: head=%v tail=%v", l.head, l.tail)
	}
}

func TestArenaListEdgeCases(t *testing.T) {
	l := New[int]()

	// Remove from empty list
	if l.Remove(&Element[int]{}) != nil {
		t.Error("Remove from empty list should return nil")
	}

	// Insert before/after a non-existent element
	if l.InsertBefore(1, &Element[int]{}) != nil {
		t.Error("InsertBefore with foreign element should return nil")
	}
	if l.InsertAfter(1, &Element[int]{}) != nil {
		t.Error("InsertAfter with foreign element should return nil")
	}

	// PushFront/PushBack into an empty list
	l2 := New[int]()
	el1 := l2.PushFront(10)
	checkListIntegrity(t, l2)
	if l2.Len() != 1 || l2.Front() != el1 || l2.Back() != el1 {
		t.Error("PushFront failed on empty list")
	}
	l2 = New[int]()
	el2 := l2.PushBack(20)
	checkListIntegrity(t, l2)
	if l2.Len() != 1 || l2.Front() != el2 || l2.Back() != el2 {
		t.Error("PushBack failed on empty list")
	}

	// Remove head/tail/single element
	l3 := New[int]()
	el3 := l3.PushBack(30)
	l3.Remove(el3)
	checkListIntegrity(t, l3)
	if l3.Len() != 0 || l3.Front() != nil || l3.Back() != nil {
		t.Error("Remove single element failed")
	}

	// Double remove
	l4 := New[int]()
	el4 := l4.PushBack(40)
	l4.Remove(el4)
	if l4.Remove(el4) != nil {
		t.Error("Double remove should return nil")
	}

	// Move non-existent element
	l5 := New[int]()
	el5 := &Element[int]{Value: 50}
	l5.MoveToFront(el5)
	l5.MoveToBack(el5)
	// Should not panic
}

func TestArenaListStress(t *testing.T) {
	l := New[int]()
	N := 100000
	for i := 0; i < N; i++ {
		l.PushBack(i)
	}
	checkListIntegrity(t, l)
	// Remove half
	cnt := 0
	for e := l.Front(); e != nil && cnt < N/2; {
		next := e.Next()
		l.Remove(e)
		e = next
		cnt++
	}
	checkListIntegrity(t, l)
}

func TestArenaListMemoryLeak(t *testing.T) {
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	l := New[int]()
	N := 100000
	for i := 0; i < N; i++ {
		l.PushBack(i)
	}
	for e := l.Front(); e != nil; {
		next := e.Next()
		l.Remove(e)
		e = next
	}
	l = nil
	runtime.GC()
	runtime.ReadMemStats(&m2)
	if m2.Alloc > m1.Alloc+10*1024*1024 { // 10MB allowance
		t.Errorf("Possible memory leak: before=%d after=%d", m1.Alloc, m2.Alloc)
	}
}

func TestChunkCleanup(t *testing.T) {
	l := New[int]()

	// Add many elements to create multiple chunks
	N := ChunkSize*3 + 50 // 3+ chunks
	var elements []*Element[int]

	for i := 0; i < N; i++ {
		e := l.PushBack(i)
		elements = append(elements, e)
	}

	// Check initial stats
	totalChunks, activeChunks, totalElements, activeElements := l.Stats()
	t.Logf("Initial: %d total chunks, %d active chunks, %d/%d elements",
		totalChunks, activeChunks, totalElements, activeElements)

	if totalChunks < 3 {
		t.Errorf("Expected at least 3 chunks, got %d", totalChunks)
	}
	if activeElements != N {
		t.Errorf("Expected %d active elements, got %d", N, activeElements)
	}

	// Remove all elements from the middle chunk (second chunk)
	// This should trigger automatic cleanup
	middleStart := ChunkSize
	middleEnd := ChunkSize * 2

	for i := middleStart; i < middleEnd; i++ {
		l.Remove(elements[i])
	}

	// Check if cleanup happened
	totalChunks2, activeChunks2, totalElements2, activeElements2 := l.Stats()
	t.Logf("After middle chunk removal: %d total chunks, %d active chunks, %d/%d elements",
		totalChunks2, activeChunks2, totalElements2, activeElements2)

	expectedActive := N - ChunkSize
	if activeElements2 != expectedActive {
		t.Errorf("Expected %d active elements, got %d", expectedActive, activeElements2)
	}

	// The middle chunk should have been cleaned up automatically
	if totalChunks2 >= totalChunks {
		t.Logf("Automatic cleanup didn't happen, trying manual CompactChunks")

		// Try manual compaction
		cleaned := l.CompactChunks()
		t.Logf("Manual compaction cleaned %d chunks", cleaned)

		totalChunks3, activeChunks3, _, _ := l.Stats()
		t.Logf("After manual compaction: %d total chunks, %d active chunks",
			totalChunks3, activeChunks3)

		if cleaned == 0 && totalChunks3 == totalChunks2 {
			t.Error("Expected some chunks to be cleaned up")
		}
	}
}

func TestChunkCleanupEdgeCases(t *testing.T) {
	l := New[int]()

	// Test cleanup on empty list
	cleaned := l.CompactChunks()
	if cleaned != 0 {
		t.Errorf("Expected 0 chunks cleaned on empty list, got %d", cleaned)
	}

	// Add one element
	e1 := l.PushBack(1)

	// Try to cleanup first chunk (should be protected)
	l.cleanupChunk(l.chunks)

	totalChunks, _, _, _ := l.Stats()
	if totalChunks != 1 {
		t.Errorf("First chunk was incorrectly cleaned up")
	}

	// Remove the element
	l.Remove(e1)

	// First chunk should still be protected
	cleaned = l.CompactChunks()
	if cleaned != 0 {
		t.Errorf("First chunk should be protected from cleanup")
	}
}

func TestMemoryLeak(t *testing.T) {
	l := New[int]()

	// Create and destroy many elements to test for leaks
	iterations := 5
	for iter := 0; iter < iterations; iter++ {
		var elements []*Element[int]

		// Add many elements
		for i := 0; i < ChunkSize*2; i++ {
			e := l.PushBack(i)
			elements = append(elements, e)
		}

		// Remove all elements
		for _, e := range elements {
			l.Remove(e)
		}

		// Force cleanup
		cleaned := l.CompactChunks()
		totalChunks, activeChunks, _, activeElements := l.Stats()

		t.Logf("Iteration %d: cleaned %d chunks, %d total chunks, %d active chunks, %d active elements",
			iter+1, cleaned, totalChunks, activeChunks, activeElements)

		if activeElements != 0 {
			t.Errorf("Memory leak detected: %d active elements remain", activeElements)
		}

		// Should have minimal chunks left (first chunk is protected)
		if totalChunks > 2 { // Some slack for edge cases
			t.Logf("Warning: %d chunks remain, possible memory bloat", totalChunks)
		}
	}
}

func BenchmarkChunkCleanup(b *testing.B) {
	l := New[int]()

	// Pre-fill with some data
	for i := 0; i < ChunkSize*5; i++ {
		l.PushBack(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.CompactChunks()
	}
}

func TestSingleChunkReuse(t *testing.T) {
	const maxElems = 256 * 3

	list := New[int]()

	// Insert elements
	var elems []*Element[int]
	for i := 0; i < maxElems; i++ {
		e := list.PushBack(i)
		elems = append(elems, e)
	}

	// Check that the list length is correct
	if list.Len() != maxElems {
		t.Fatalf("expected length %d, got %d", maxElems, list.Len())
	}

	// Check that there are exactly 1 chunk
	totalChunks, _, _, _ := list.Stats()
	if totalChunks != 3 {
		t.Fatalf("expected 3 chunk, got %d", totalChunks)
	}

	// Remove all elements
	for _, e := range elems {
		list.Remove(e)
	}

	if list.Len() != 0 {
		t.Fatalf("expected length 0 after removal, got %d", list.Len())
	}

	// Try to insert again
	for i := 0; i < 256; i++ {
		list.PushBack(i)
	}

	// Make sure the chunk is not recreated
	newTotalChunks, _, _, _ := list.Stats()
	if newTotalChunks != 1 {
		t.Fatalf("expected chunk reuse, got %d chunks after reinsertion", newTotalChunks)
	}
}

func TestChunkAllocationAfterPartialRemove(t *testing.T) {
	list := New[string]()

	// Add exactly ChunkSize elements to fill the first chunk
	for i := 0; i < ChunkSize; i++ {
		list.PushBack("elem")
	}

	// Remove elements from the beginning (leave 15 at the end)
	for i := 0; i < ChunkSize-15; i++ {
		list.Remove(list.Front())
	}

	if list.Len() != 15 {
		t.Fatalf("Expected length 15 after removals, got %d", list.Len())
	}

	totalChunksBefore, _, _, _ := list.Stats()

	// Add a new element - a NEW chunk should be created
	// because arena allocation works as a bump allocator
	newElem := list.PushBack("new_elem")

	if newElem == nil {
		t.Fatal("Expected new element to be allocated")
	}

	totalChunksAfter, _, _, _ := list.Stats()

	// Check that a new chunk was created (correct behavior for arena)
	if totalChunksAfter != totalChunksBefore+1 {
		t.Errorf("Expected new chunk to be created due to arena allocation principles, got %d chunks before and %d after",
			totalChunksBefore, totalChunksAfter)
	}

	// Проверяем что элемент действительно добавился
	if list.Len() != 16 {
		t.Fatalf("Expected length 16 after adding new element, got %d", list.Len())
	}
}

// func TestArenaListVisualDemo(t *testing.T) {
// 	fmt.Println("=== Visual demonstration of arenalist and chunks ===")
// 	l := New[int]()
// 	values := make([]int, 300)
// 	for i := 0; i < 300; i++ {
// 		values[i] = i + 1
// 	}
// 	var elems []*Element[int]

// 	// Add elements to create several chunks
// 	for i, v := range values {
// 		e := l.PushBack(v)
// 		elems = append(elems, e)
// 		fmt.Printf("PushBack '%d' → Element at chunk %p idx %d\n", v, e.Chunk(), e.Index())
// 		if (i+1)%ChunkSize == 0 {
// 			fmt.Printf("--- Chunk is full, a new one will be created on the next insert ---\n")
// 		}
// 	}

// 	fmt.Println("\n=== List of elements and their chunks ===")
// 	for e := l.Front(); e != nil; e = e.Next() {
// 		fmt.Printf("Value: %d | chunk: %p | idx: %d | prev: %p | next: %p\n", e.Value, e.Chunk(), e.Index(), e.Prev(), e.Next())
// 	}

// 	fmt.Println("\n=== Chunk structure ===")
// 	for c := l.Chunks(); c != nil; c = c.next {
// 		fmt.Printf("Chunk %p: ", c)
// 		for i := 0; i < c.used; i++ {
// 			el := &c.elements[i]
// 			fmt.Printf("[%d idx:%d] ", el.Value, el.index)
// 		}
// 		fmt.Printf("→ next: %p\n", c.next)
// 	}

// 	fmt.Println("\n=== Checking element links (next/prev) ===")
// 	for _, e := range elems {
// 		prevVal := 0
// 		nextVal := 0
// 		if e.Prev() != nil {
// 			prevVal = e.Prev().Value
// 		}
// 		if e.Next() != nil {
// 			nextVal = e.Next().Value
// 		}
// 		fmt.Printf("Element '%d': prev = %d, next = %d\n", e.Value, prevVal, nextVal)
// 	}

// 	fmt.Println("\n=== Summary: head/tail/len ===")
// 	fmt.Printf("Head: %d\n", l.Front().Value)
// 	fmt.Printf("Tail: %d\n", l.Back().Value)
// 	fmt.Printf("Len: %d\n", l.Len())
// }

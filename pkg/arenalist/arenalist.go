package arenalist

// Chunk size tuned for cache lines
const ChunkSize = 256

// Element - list element, compatible with container/list.
// Memory layout aware structure optimized for fast access
type Element[T any] struct {
	// Hot fields first - accessed in every operation
	next, prev *Element[T] // 16 bytes - pointers

	// Cold fields - used less frequently, grouped together
	list  *List[T]  // 8 bytes - only for validation
	chunk *chunk[T] // 8 bytes - only for allocation tracking
	index int       // 8 bytes - only for debugging
	// Generation used to invalidate stale pointers across compactions
	generation int

	// Value field last - may be large and accessed less frequently
	Value T // Variable size - the actual data
}

// List optimized for cache performance and hot path operations
type List[T any] struct {
	// Hot fields first - used constantly
	head, tail *Element[T] // 16 bytes - critical for all operations
	length     int         // 8 bytes - checked frequently
	generation int         // 8 bytes - increments on compaction

	// Cold fields - arena management
	last   *chunk[T] // 8 bytes - only for allocation
	chunks *chunk[T] // 8 bytes - only for cleanup/iteration
}

// chunk optimized for fast memory allocation
type chunk[T any] struct {
	used   int       // 8 bytes - updated often during allocation
	active int       // 8 bytes - count of non-removed elements for GC
	next   *chunk[T] // 8 bytes - for chunk traversal

	elements [ChunkSize]Element[T]
}

func New[T any]() *List[T] {
	return &List[T]{}
}

// Hot methods - should inline for performance
func (l *List[T]) Len() int {
	return l.length
}

func (l *List[T]) Front() *Element[T] {
	return l.head
}

func (l *List[T]) Back() *Element[T] {
	return l.tail
}

func (e *Element[T]) Next() *Element[T] {
	return e.next
}

func (e *Element[T]) Prev() *Element[T] {
	return e.prev
}

// PushFront - fast insertion at the front
func (l *List[T]) PushFront(v T) *Element[T] {
	e := l.alloc(v)

	if head := l.head; head != nil {
		e.next = head
		head.prev = e
	} else {
		l.tail = e
	}

	l.head = e
	l.length++
	return e
}

// PushBack - fast insertion at the back
func (l *List[T]) PushBack(v T) *Element[T] {
	e := l.alloc(v)

	if tail := l.tail; tail != nil {
		e.prev = tail
		tail.next = e
	} else {
		l.head = e
	}

	l.tail = e
	l.length++
	return e
}

func (l *List[T]) InsertBefore(v T, mark *Element[T]) *Element[T] {
	if mark == nil || mark.list != l || mark.generation != l.generation {
		return nil
	}
	e := l.alloc(v)
	e.prev = mark.prev
	e.next = mark
	mark.prev = e
	if e.prev != nil {
		e.prev.next = e
	} else {
		l.head = e
	}
	l.length++
	return e
}

func (l *List[T]) InsertAfter(v T, mark *Element[T]) *Element[T] {
	if mark == nil || mark.list != l || mark.generation != l.generation {
		return nil
	}
	e := l.alloc(v)
	e.next = mark.next
	e.prev = mark
	mark.next = e
	if e.next != nil {
		e.next.prev = e
	} else {
		l.tail = e
	}
	l.length++
	return e
}

// Remove - fast removal with validation for public API
func (l *List[T]) Remove(e *Element[T]) *Element[T] {
	if e == nil || e.list != l || e.list == nil || e.generation != l.generation {
		return nil
	}

	if prev := e.prev; prev != nil {
		prev.next = e.next
	} else {
		l.head = e.next
	}

	if next := e.next; next != nil {
		next.prev = e.prev
	} else {
		l.tail = e.prev
	}

	// Decrease active count and maybe cleanup chunk
	if e.chunk != nil {
		if e.chunk.active > 0 {
			e.chunk.active--
		}
		// Reset chunk for reuse when it becomes empty
		if e.chunk.active == 0 {
			e.chunk.used = 0 // Reset for reuse
			// Only cleanup non-essential chunks (not first or last)
			if e.chunk != l.chunks && e.chunk != l.last {
				l.cleanupChunk(e.chunk)
			}
		}
	}

	// Clear references for GC and mark as removed
	e.next = nil
	e.prev = nil
	e.list = nil // Mark as removed for double-remove protection
	e.chunk = nil
	// Keep Value for caller

	l.length--
	// If list is empty, collapse all chunks into a single reusable first chunk
	if l.length == 0 && l.chunks != nil {
		// Ensure the first chunk is kept and detached from the rest
		first := l.chunks
		// Reset counters for clean reuse
		first.used = 0
		first.active = 0
		// Drop the rest of the chain for GC
		first.next = nil
		l.last = first
	}
	return e
}

// MoveToFront - optimized movement without allocation
func (l *List[T]) MoveToFront(e *Element[T]) {
	// Validation and fast path
	if e == nil || e.list != l || e.generation != l.generation || l.head == e {
		return
	}

	if prev := e.prev; prev != nil {
		prev.next = e.next
	}
	if next := e.next; next != nil {
		next.prev = e.prev
	} else {
		l.tail = e.prev
	}

	e.prev = nil
	if head := l.head; head != nil {
		e.next = head
		head.prev = e
	} else {
		e.next = nil
		l.tail = e
	}
	l.head = e
}

// MoveToBack - optimized movement without allocation
func (l *List[T]) MoveToBack(e *Element[T]) {
	// Validation and fast path
	if e == nil || e.list != l || e.generation != l.generation || l.tail == e {
		return
	}

	if prev := e.prev; prev != nil {
		prev.next = e.next
	} else {
		l.head = e.next
	}
	if next := e.next; next != nil {
		next.prev = e.prev
	}

	e.next = nil
	if tail := l.tail; tail != nil {
		e.prev = tail
		tail.next = e
	} else {
		e.prev = nil
		l.head = e
	}
	l.tail = e
}

// alloc - optimized allocation for hot paths
func (l *List[T]) alloc(val T) *Element[T] {
	if last := l.last; last != nil && last.used < ChunkSize {
		i := last.used
		last.used++
		e := &last.elements[i]

		// Minimal field initialization for hot path
		e.Value = val
		e.list = l
		e.chunk = last
		e.index = i
		e.generation = l.generation
		e.next = nil
		e.prev = nil

		// Track active elements for GC
		last.active++

		return e
	}

	// Slow path - need new chunk (unlikely)
	return l.allocSlow(val)
}

// allocSlow - separate function for rare chunk allocation
//
//go:noinline
func (l *List[T]) allocSlow(val T) *Element[T] {
	c := &chunk[T]{}
	if l.last != nil {
		l.last.next = c
	} else {
		l.chunks = c
	}
	l.last = c

	// Allocate first element in new chunk
	e := &c.elements[0]
	c.used = 1
	c.active = 1 // First active element

	e.Value = val
	e.list = l
	e.chunk = c
	e.index = 0
	e.generation = l.generation
	e.next = nil
	e.prev = nil

	return e
}

// Cold methods - only for debugging/introspection
func (e *Element[T]) Chunk() *chunk[T] {
	return e.chunk
}

func (e *Element[T]) Index() int {
	return e.index
}

func (l *List[T]) Chunks() *chunk[T] {
	return l.chunks
}

func (l *List[T]) LastChunk() *chunk[T] {
	return l.last
}

// cleanupChunk - aggressive cleanup of empty chunks to prevent memory leaks
func (l *List[T]) cleanupChunk(targetChunk *chunk[T]) {
	if targetChunk == nil || targetChunk.active > 0 {
		return // Chunk still has active elements
	}

	// Don't cleanup first chunk or last chunk
	if targetChunk == l.chunks || targetChunk == l.last {
		return
	}

	// Find and unlink the chunk from the chain
	var prev *chunk[T]
	for current := l.chunks; current != nil; current = current.next {
		if current == targetChunk {
			// Found the chunk to remove
			if prev != nil {
				prev.next = current.next
			} else {
				// This shouldn't happen since we checked != l.chunks above
				l.chunks = current.next
			}

			// Clear the chunk for GC
			current.next = nil

			break
		}
		prev = current
	}
}

// CompactChunks - periodic cleanup of all empty chunks
// Call this occasionally to prevent memory fragmentation
func (l *List[T]) CompactChunks() int {
	if l.chunks == nil {
		return 0
	}

	cleaned := 0
	var prev *chunk[T]
	current := l.chunks

	for current != nil {
		next := current.next

		// Can't cleanup first or last chunk, and must have no active elements
		if current.active == 0 && current != l.chunks && current != l.last {
			// Unlink the chunk
			if prev != nil {
				prev.next = next
			}

			// Clear for GC
			current.next = nil
			cleaned++

			// Don't advance prev since we removed current
		} else {
			prev = current
		}

		current = next
	}

	return cleaned
}

// Stats - debug information about chunks
func (l *List[T]) Stats() (totalChunks, activeChunks, totalElements, activeElements int) {
	for c := l.chunks; c != nil; c = c.next {
		totalChunks++
		totalElements += c.used
		activeElements += c.active
		if c.active > 0 {
			activeChunks++
		}
	}
	return
}

// DebugChunks - detailed debug information about all chunks
func (l *List[T]) DebugChunks() []ChunkInfo {
	var infos []ChunkInfo
	chunkIndex := 0
	for c := l.chunks; c != nil; c = c.next {
		info := ChunkInfo{
			Index:  chunkIndex,
			Used:   c.used,
			Active: c.active,
		}
		infos = append(infos, info)
		chunkIndex++
	}
	return infos
}

// ChunkInfo - debug information about a single chunk
type ChunkInfo struct {
	Index  int
	Used   int
	Active int
}

func (l *List[T]) PushFrontBatch(values []T) {
	for i := len(values) - 1; i >= 0; i-- {
		l.PushFront(values[i])
	}
}

func (l *List[T]) PushBackBatch(values []T) {
	for _, v := range values {
		l.PushBack(v)
	}
}

func (l *List[T]) Preallocate(count int) {
	chunksNeeded := (count + ChunkSize - 1) / ChunkSize
	for i := 0; i < chunksNeeded; i++ {
		c := &chunk[T]{}
		if l.last != nil {
			l.last.next = c
		} else {
			l.chunks = c
		}
		l.last = c
	}
}

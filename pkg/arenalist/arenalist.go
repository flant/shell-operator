package arenalist

// Chunk size tuned for cache lines
const ChunkSize = 256

// Element - list element, compatible with container/list.
// Memory layout aware structure optimized for fast access
type Element[T any] struct {
	next, prev *Element[T] // 16 bytes - pointers
	Value      T           // Variable size - the actual data

	// Cold fields - used less frequently
	list  *List[T]  // 8 bytes - only for validation
	chunk *chunk[T] // 8 bytes - only for allocation tracking
	index int       // 8 bytes - only for debugging
}

// List optimized for cache performance and hot path operations
type List[T any] struct {
	// Hot fields first - used constantly
	head, tail *Element[T] // 16 bytes - critical for all operations
	length     int         // 8 bytes - checked frequently

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
	if mark.list != l {
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
	if mark.list != l {
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
	if e == nil || e.list != l || e.list == nil {
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
		e.chunk.active--
		// Reset chunk for reuse when it becomes empty
		if e.chunk.active == 0 {
			e.chunk.used = 0 // Reset for reuse!

			// Special case: if both first and last chunks are empty, merge them
			if l.chunks != l.last && l.chunks.active == 0 && l.last.active == 0 {
				// Make first chunk the only chunk
				l.last = l.chunks
				// Clean up the old last chunk
				l.сleanupсhunk(e.chunk)
			} else if e.chunk != l.chunks && e.chunk != l.last {
				// Only cleanup non-essential chunks (not first or last)
				l.сleanupсhunk(e.chunk)
			}
		}
	}

	// Clear references for GC and mark as removed
	e.next = nil
	e.prev = nil
	e.list = nil // Mark as removed for double-remove protection
	// Keep Value for caller

	l.length--
	return e
}

// MoveToFront - optimized movement without allocation
func (l *List[T]) MoveToFront(e *Element[T]) {
	// Validation and fast path
	if e == nil || e.list != l || l.head == e {
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
	if e == nil || e.list != l || l.tail == e {
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

// maybeCleanupChunk - aggressive cleanup of empty chunks to prevent memory leaks
func (l *List[T]) сleanupсhunk(targetChunk *chunk[T]) {
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

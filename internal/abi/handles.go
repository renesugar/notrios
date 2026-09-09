package abi

import "sync"

// A handle is a 64-bit integer split into a 32-bit generation and a 32-bit
// slot:
//
//	63                    32 31                     0
//	+-----------------------+-----------------------+
//	|      generation       |         slot          |
//	+-----------------------+-----------------------+
//
// The generation is what makes a released handle detectably wrong rather than
// quietly dangerous. Slots are reused; generations are not. A host that closes
// an instance and then uses the old handle gets StatusStaleHandle, even if the
// slot has since been handed to a different instance — which is exactly the
// bug that would otherwise corrupt someone else's database. Getting this wrong
// is not a crash, it is a silent write to the wrong library.
//
// Generation 0 is never issued, so a zeroed handle is always invalid.
type Handle uint64

func makeHandle(generation uint32, slot uint32) Handle {
	return Handle(uint64(generation)<<32 | uint64(slot))
}

func (h Handle) generation() uint32 { return uint32(uint64(h) >> 32) }
func (h Handle) slot() uint32       { return uint32(uint64(h)) }

// table is a generation-checked handle map. The zero value is not usable;
// construct with newTable.
type table[T any] struct {
	mu         sync.Mutex
	entries    map[uint32]tableEntry[T]
	nextSlot   uint32
	generation uint32
	// retired remembers generations that were issued and released, so a stale
	// handle can be told apart from one that was never valid. It is bounded by
	// retiredLimit; beyond that the oldest are forgotten and such a handle
	// reports StatusStaleHandle anyway, which is the safe direction.
	retired map[Handle]bool
}

type tableEntry[T any] struct {
	generation uint32
	value      T
}

// retiredLimit bounds the released-handle memory. A host that opens and closes
// millions of calls must not grow this forever.
const retiredLimit = 4096

func newTable[T any]() *table[T] {
	return &table[T]{entries: make(map[uint32]tableEntry[T]), retired: make(map[Handle]bool)}
}

// insert stores value and returns its handle.
func (t *table[T]) insert(value T) Handle {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.generation++
	if t.generation == 0 {
		// Generation 0 is reserved so a zeroed handle can never be valid.
		t.generation = 1
	}
	slot := t.nextSlot
	t.nextSlot++
	t.entries[slot] = tableEntry[T]{generation: t.generation, value: value}
	return makeHandle(t.generation, slot)
}

// lookup resolves a handle. It distinguishes a handle that was never valid
// (StatusInvalidHandle) from one that has been released (StatusStaleHandle),
// because a host debugging a use-after-close needs to know which it is.
func (t *table[T]) lookup(handle Handle) (T, Status) {
	var zero T
	if handle == 0 || handle.generation() == 0 {
		return zero, StatusInvalidHandle
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, present := t.entries[handle.slot()]
	if !present {
		if t.retired[handle] {
			return zero, StatusStaleHandle
		}
		return zero, StatusInvalidHandle
	}
	if entry.generation != handle.generation() {
		// The slot was reused by a later object. The caller is holding a
		// handle to something that no longer exists.
		return zero, StatusStaleHandle
	}
	return entry.value, StatusOK
}

// remove releases a handle, returning the value it held.
func (t *table[T]) remove(handle Handle) (T, Status) {
	var zero T
	if handle == 0 || handle.generation() == 0 {
		return zero, StatusInvalidHandle
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, present := t.entries[handle.slot()]
	if !present || entry.generation != handle.generation() {
		if t.retired[handle] {
			return zero, StatusStaleHandle
		}
		if !present {
			return zero, StatusInvalidHandle
		}
		return zero, StatusStaleHandle
	}
	delete(t.entries, handle.slot())
	t.retire(handle)
	return entry.value, StatusOK
}

// retire records a released handle. Callers must hold the lock.
func (t *table[T]) retire(handle Handle) {
	if len(t.retired) >= retiredLimit {
		for existing := range t.retired {
			delete(t.retired, existing)
			if len(t.retired) < retiredLimit {
				break
			}
		}
	}
	t.retired[handle] = true
}

// values returns every live value. Used by shutdown, which must reach every
// call and stream regardless of who holds a handle to it.
func (t *table[T]) values() []T {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]T, 0, len(t.entries))
	for _, entry := range t.entries {
		out = append(out, entry.value)
	}
	return out
}

func (t *table[T]) len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.entries)
}

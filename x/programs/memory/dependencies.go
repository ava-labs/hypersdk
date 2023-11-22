package memory

// Memory defines the interface for interacting with memory.
type MemoryRemove interface {
	// Range returns an owned slice of data from a specified offset.
	Range(uint64, uint64) ([]byte, error)
	// Alloc allocates a block of memory and returns a pointer
	// (offset) to its location on the stack.
	Alloc(uint64) (uint64, error)
	// Write writes the given data to the memory at the given offset.
	Write(uint64, []byte) error
	// Len returns the length of this memory in bytes.
	Len() (uint64, error)
	// Grow increases the size of the memory pages by delta.
	Grow(uint64) (uint64, error)
}

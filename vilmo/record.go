package vilmo

type record struct {
	log uint64
	key string

	// Only populated if the value is less than [minDiskValueSize]
	value []byte

	// Location of value in file (does not include
	// operation, key length, key, or value length)
	loc  int64
	size uint32

	// interleaved (across batches) doubly-linked list allows for removals
	prev *record
	next *record
}

func (r *record) Size() int64 {
	if r.loc >= 0 {
		return int64(r.size)
	}
	return int64(len(r.value))
}

func (r *record) Cached() bool {
	return r.loc < 0
}

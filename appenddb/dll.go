package appenddb

type record struct {
	batch uint64
	key   string

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

type dll struct {
	first *record
	last  *record
}

func (d *dll) Add(r *record) {
	// Clean record
	r.prev = nil
	r.next = nil

	// Add to linked list
	if d.first == nil {
		d.first = r
		d.last = r
		return
	}
	r.prev = d.last
	d.last.next = r
	d.last = r
}

func (d *dll) Remove(r *record) {
	// Remove from linked list
	if r.prev == nil {
		d.first = r.next
	} else {
		r.prev.next = r.next
	}
	if r.next == nil {
		d.last = r.prev
	} else {
		r.next.prev = r.prev
	}
	if r.next == r {
		panic("next is self")
	}
}

type dllIterator struct {
	cursor *record
}

func (d *dll) Iterator() *dllIterator {
	return &dllIterator{cursor: d.first}
}

func (d *dllIterator) Next() *record {
	if d.cursor == nil {
		return nil
	}
	r := d.cursor
	d.cursor = d.cursor.next
	return r
}

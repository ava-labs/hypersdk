package vilmo

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

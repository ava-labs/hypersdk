package vilmo

type record struct {
	// log is the log file that contains this record
	//
	// By storing a pointer here, we can avoid a map
	// lookup if we need to access this log.
	log *log

	// key is the length of the key
	key string

	// Only populated if the value is less than [minDiskValueSize]
	cached bool
	value  []byte

	// loc is the offset of the record in the log file
	//
	// We store the beginning of the record here for using
	// in nullify operations.
	loc int64
	// size is the size fo the value in the log file
	size uint32

	// interleaved (across batches) doubly-linked list allows for removals
	prev *record
	next *record
}

func (r *record) Size() uint32 {
	return r.size
}

// ValueLoc returns the locaction of the value in the log file
func (r *record) ValueLoc() int64 {
	return r.loc + opPutToValue(r.key)
}

package vilmo

import (
	"github.com/ava-labs/avalanchego/ids"
	"golang.org/x/exp/mmap"
)

// log files store all operations for vilmo.
//
// They are structured as repeated sequences of:
// [opBatch][opPut/opDelete]...[opPut/opDelete][opChecksum]...
type log struct {
	// reader must be reopened anytime the underlying file
	// is appended to.
	reader *mmap.ReaderAt

	// aliveBytes/uselessBytes are used to determine when to rewrite
	// a log file.
	aliveBytes   int64
	uselessBytes int64 // already includes all overwritten data, checksums, and deletes

	// checksum is the last checkpoint in a log file.
	checksum ids.ID

	// alive is used to determine the order to write living
	// data in the case a log needs to be rewritten.
	alive *dll
}

// we return the record to allow for memory reuse + less map ops
func (l *log) Remove(record *record) {
	opSize := opPutLenWithValueLen(record.key, record.Size())
	l.aliveBytes -= opSize
	l.uselessBytes += opSize

	// Remove from linked list
	l.alive.Remove(record)
}

func load(path string) (uint64, ids.ID, []op, error) {
}

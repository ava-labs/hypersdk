package vilmo

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
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

	// pendingNullify is a list of put records that have been deleted
	// on other log files. In the case that this log file is recycled,
	// we must persist these nullifications to ensure we can restore state
	// on restart.
	pendingNullify []int64
}

func (l *log) Add(record *record) {
	opSize := opPutLenWithValueLen(record.key, record.Size())
	l.aliveBytes += opSize

	// Add to linked list
	l.alive.Add(record)
}

// TODO: handle nullify bytes included
func (l *log) Remove(record *record, nullify bool) {
	opSize := opPutLenWithValueLen(record.key, record.Size())
	l.aliveBytes -= opSize
	l.uselessBytes += opSize

	// Remove from linked list
	l.alive.Remove(record)

	// We should only nullify a record if the update/delete is on another log. If it is
	// on the same log file, we don't need to nullify it.
	if nullify {
		l.pendingNullify = append(l.pendingNullify, record.loc)
	}
}

// reader tracks how many bytes we read of a file to support
// arbitrary truncation.
type reader struct {
	cursor int64
	reader *bufio.Reader
}

func (r *reader) Cursor() int64 {
	return r.cursor
}

func (r *reader) Read(p []byte) error {
	n, err := io.ReadFull(r.reader, p)
	r.cursor += int64(n)
	return err
}

type batchIndex struct {
	batch uint64
	index int
}

type putOp struct {
	key    string
	record *record
}

type deleteOp struct {
	key string
	log *log
}

// load will attempt to load a log file from disk.
//
// If a batch is partially written or corrupt, the batch will be removed
// from the log file and the last non-corrupt batch will be returned. If
// there are no non-corrupt batches the file will be deleted and a nil *log
// will be retunred.
//
// Partial batch writing should not occur unless there is an unclean shutdown,
// as the usage of [Abort] prevents this.
func load(logger logging.Logger, logNum uint64, path string) (*log, map[uint64][]any, error) {
	// Open log file
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	fileSize := int64(fi.Size())

	// Read log file ops
	var (
		reader = &reader{reader: bufio.NewReader(f)}

		// We create the log here so that any read items can reference it.
		l = &log{alive: &dll{}}

		lastBatch uint64
		batchSet  bool
		keys      = map[int64]*batchIndex{}

		hasher       = sha256.New()
		ops          []any
		uselessBytes int64

		committedByte         int64
		committedChecksum     ids.ID
		committedUselessBytes int64
		committedOps          = map[uint64][]any{}

		corrupt error
	)
	for {
		start := reader.Cursor()
		opType, err := readOpType(reader, hasher)
		if err != nil {
			corrupt = err
			break
		}
		if !batchSet && opType != opBatch {
			corrupt = fmt.Errorf("expected batch op but got %d", opType)
			break
		}
		switch opType {
		case opPut:
			key, value, err := readPut(reader, hasher)
			if err != nil {
				corrupt = err
				break
			}
			r := &record{
				log: l,
				key: uint16(len(key)),

				loc:  start,
				size: uint32(len(value)),
			}
			if len(value) < minDiskValueSize {
				r.cached = true
				r.value = value
			}
			ops = append(ops, r)
			keys[start] = &batchIndex{batch: lastBatch, index: len(ops) - 1}

			// We wait to adjust [aliveBytes] until we know if a value is actually
			// added to [keys].
		case opDelete:
			del, err := readDelete(reader, hasher)
			if err != nil {
				corrupt = err
				break
			}
			uselessBytes += opDeleteLen(del)
			ops = append(ops, &deleteOp{del, l})
		case opBatch:
			batch, err := readBatch(reader, hasher)
			if err != nil {
				corrupt = err
				break
			}
			uselessBytes += opBatchLen()
			if batchSet {
				corrupt = fmt.Errorf("batch %d already set", lastBatch)
				break
			}
			lastBatch = batch
			batchSet = true
			ops = []any{}
		case opChecksum:
			checksum, err := readChecksum(reader)
			if err != nil {
				corrupt = err
				break
			}
			uselessBytes += opChecksumLen()
			computed := ids.ID(hasher.Sum(nil))
			if checksum != computed {
				corrupt = fmt.Errorf("checksum mismatch expected=%d got=%d", checksum, computed)
				break
			}

			// Update our track for last committed
			committedByte = reader.Cursor()
			committedChecksum = checksum
			committedUselessBytes = uselessBytes
			committedOps[lastBatch] = ops

			// Check if we should exit (only clean exit)
			if reader.Cursor() == fileSize {
				break
			}

			// Keep reading
			hasher = sha256.New()
			if _, err := hasher.Write(checksum[:]); err != nil {
				corrupt = err
				break
			}
			batchSet = false
			ops = nil
		case opNullify:
			loc, err := readNullify(reader, hasher)
			if err != nil {
				corrupt = err
				break
			}
			uselessBytes += opNullifyLen()
			bi, ok := keys[loc]
			if !ok {
				corrupt = fmt.Errorf("nullify key not found at %d", loc)
				break
			}

			// It is not possible to nullify a put operation in the same
			// batch, so this will not panic.
			committedOps[bi.batch][bi.index] = nil
		default:
			corrupt = fmt.Errorf("unknown op type %d", opType)
			break
		}
	}

	// Close file once we are done reading
	if err := f.Close(); err != nil {
		return nil, nil, err
	}

	// If the log file is corrupt, attempt to revert
	// to the last non-corrupt op.
	if corrupt != nil {
		logger.Warn(
			"log file is corrupt",
			zap.String("path", path),
			zap.Error(corrupt),
		)
	}

	// If after recovery the log file is empty, return to caller.
	if committedByte == 0 {
		// Remove the empty file
		if err := os.Remove(path); err != nil {
			return nil, nil, fmt.Errorf("%w: unable to remove useless file", err)
		}
		logger.Warn(
			"removing corrupt log",
			zap.String("path", path),
		)
		return nil, nil, nil
	} else {
		if err := os.Truncate(path, committedByte); err != nil {
			return nil, nil, fmt.Errorf("%w: unable to truncate file", err)
		}
		logger.Warn(
			"truncating corrupt log",
			zap.String("path", path),
			zap.Int64("tip", fileSize),
			zap.Int64("committed", committedByte),
		)
	}

	// Open file for mmap
	m, err := mmap.Open(path)
	if err != nil {
		return nil, nil, err
	}
	l.reader = m
	l.uselessBytes = committedUselessBytes
	l.checksum = committedChecksum
	return l, committedOps, nil
}

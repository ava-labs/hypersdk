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
}

func (l *log) Remove(record *record) {
	opSize := opPutLenWithValueLen(record.key, record.Size())
	l.aliveBytes -= opSize
	l.uselessBytes += opSize

	// Remove from linked list
	l.alive.Remove(record)
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

type pendingLog struct {
	Batch    uint64
	Checksum ids.ID
	Ops      []op

	// UselessBytes only includes bytes from [opBatch] and [opChecksum]
	// because we do not yet know if any [opPut] or [opDelete] are useless.
	UselessBytes int64
}

// load will attempt to load a log file from disk.
//
// If a batch is partially written or corrupt, the batch will be removed
// from the log file and the last non-corrupt batch will be returned. If
// there are no non-corrupt batches the file will be deleted and a nil
// [pendingLog] will be returned.
//
// Partial batch writing should not occur unless there is an unclean shutdown,
// as the usage of [Abort] prevents this.
func load(logger logging.Logger, path string) (*pendingLog, error) {
	// Open log file
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := int64(fi.Size())

	// Read log file ops
	var (
		reader = &reader{reader: bufio.NewReader(f)}

		lastBatch uint64
		batchSet  bool

		hasher       = sha256.New()
		ops          = []op{}
		uselessBytes int64

		lastCommitByte     int64
		lastCommitBatch    uint64
		lastCommitChecksum ids.ID
		lastCommitOps      int
		lastUselessBytes   int64

		corrupt error
	)
	for {
		op, err := readOpType(reader, hasher)
		if err != nil {
			corrupt = err
			break
		}
		if !batchSet && op != opBatch {
			corrupt = fmt.Errorf("expected batch op but got %d", op)
			break
		}
		switch op {
		case opPut:
			put, err := readPut(reader, hasher)
			if err != nil {
				corrupt = err
				break
			}
			put.SetBatch(lastBatch)
			ops = append(ops, put)
		case opDelete:
			del, err := readDelete(reader, hasher)
			if err != nil {
				corrupt = err
				break
			}
			del.SetBatch(lastBatch)
			ops = append(ops, del)
		case opBatch:
			uselessBytes += opBatchLen()
			batch, err := readBatch(reader, hasher)
			if err != nil {
				corrupt = err
				break
			}
			if batchSet {
				corrupt = fmt.Errorf("batch %d already set", lastBatch)
				break
			}
			lastBatch = batch
			batchSet = true
		case opChecksum:
			uselessBytes += opChecksumLen()
			checksum, err := readChecksum(reader)
			if err != nil {
				corrupt = err
				break
			}
			computed := ids.ID(hasher.Sum(nil))
			if checksum != computed {
				corrupt = fmt.Errorf("checksum mismatch expected=%d got=%d", checksum, computed)
				break
			}

			// Update our track for last committed
			lastCommitByte = reader.Cursor()
			lastCommitBatch = lastBatch
			lastCommitChecksum = checksum
			lastCommitOps = len(ops)
			lastUselessBytes = uselessBytes

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
		}
	}

	// Close file once we are done reading
	if err := f.Close(); err != nil {
		return nil, err
	}

	// If the log file is corrupt, attempt to revert
	// to the last non-corrupt op.
	if corrupt != nil {
		logger.Warn(
			"log file is corrupt",
			zap.String("path", path),
			zap.Error(corrupt),
			zap.Int("tip", len(ops)),
			zap.Int("committed", lastCommitOps),
		)
		ops = ops[:lastCommitOps]
	}

	// If after recovery the log file is empty, return to caller.
	if lastCommitByte == 0 {
		// Remove the empty file
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("%w: unable to remove useless file", err)
		}
		logger.Warn(
			"log file is empty",
			zap.String("path", path),
		)
		return nil, nil
	} else {
		if err := os.Truncate(path, lastCommitByte); err != nil {
			return nil, fmt.Errorf("%w: unable to truncate file", err)
		}
	}
	return &pendingLog{
		Batch:        lastCommitBatch,
		Checksum:     lastCommitChecksum,
		Ops:          ops,
		UselessBytes: lastUselessBytes,
	}, nil
}

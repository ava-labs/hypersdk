package vilmo

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"go.uber.org/zap"
	"golang.org/x/exp/mmap"
)

var _ state.Mutable = (*Batch)(nil)

// Batch is not thread-safe
//
// Batch assumes that keys are inserted in a **deterministic** order
// otherwise the checksum will differ across nodes.
//
// Batch assumes there is only one interaction per key per batch.
type Batch struct {
	a     *Vilmo
	path  string
	batch uint64

	pruneableBatch *uint64
	openWrites     int64 // bytes
	movingPath     string
	startingCursor int64

	hasher hash.Hash
	f      *os.File
	cursor int64

	buf    []byte
	writer *writer

	t *tracker
}

func (a *Vilmo) NewBatch() (*Batch, error) {
	a.commitLock.Lock()
	batch := a.nextBatch
	path := filepath.Join(a.baseDir, strconv.FormatUint(batch, 10))
	a.nextBatch++
	b := &Batch{
		a:     a,
		path:  path,
		batch: batch,

		hasher: sha256.New(),

		buf: make([]byte, batchBufferSize),
	}

	// If we don't need to recycle, we should create a new hashmap for this
	// batch.
	reused, err := b.recycle()
	if err != nil {
		_ = b.f.Close()
		// We don't remove this file because it may be a reuse file that we want to recover
		return nil, err
	}
	if reused {
		return b, nil
	}

	// Create new file
	f, err := os.Create(b.path)
	if err != nil {
		return nil, err
	}
	b.f = f
	b.writer = newWriter(f, 0, b.a.bufferSize)
	b.t.alive = &dll{}
	if err := b.writeBatch(); err != nil {
		return nil, err
	}
	opLen := opBatchLen()
	b.openWrites += opLen
	b.t.uselessBytes += opLen
	return b, nil
}

func (b *Batch) writeBuffer(value []byte, hash bool) error {
	b.writer.Write(value)
	if hash {
		if _, err := b.hasher.Write(value); err != nil {
			return err
		}
	}
	b.cursor += int64(len(value))
	return nil
}

// recycle reads all keys from the oldest batch
// and puts them into the new file.
//
// A byproduct of this approach is that batches may contain duplicate
// keys (if a key is replaced in this batch), however, we are willing to make
// that tradeoff to begin writing the new batch as soon as possible.
//
// It is safe to call prepare multiple times.
func (b *Batch) recycle() (bool, error) {
	b.a.logger.Debug("recycling previous batch file", zap.Uint64("batch", b.batch))

	// Determine if we should delete the oldest batch
	if b.batch < uint64(b.a.historyLen) {
		return false, nil
	}

	previous := b.a.batches[oldestBatch]
	b.pruneableBatch = &oldestBatch

	// Determine if we should continue writing to the file or create a new one
	if previous.aliveBytes < previous.uselessBytes/uselessDividerRecycle || previous.uselessBytes > forceRecycle {
		b.a.logger.Debug("rewriting alive data to a new file", zap.Int64("alive bytes", previous.aliveBytes), zap.Int64("useless bytes", previous.uselessBytes), zap.Uint64("batch", b.batch))

		// Create new file
		f, err := os.Create(b.path)
		if err != nil {
			return false, err
		}
		b.f = f
		b.writer = newWriter(f, 0, b.a.bufferSize)
		if err := b.writeBatch(); err != nil {
			return false, err
		}
		opLen := opBatchLen()
		b.openWrites += opLen
		b.t.uselessBytes += opLen

		// Iterate over alive records and add them to the batch file
		b.t.alive = &dll{}
		iter := previous.alive.Iterator()
		for next := iter.Next(); next != nil; next = iter.Next() {
			value, err := b.a.Get(context.TODO(), next.key)
			if err != nil {
				return false, err
			}

			// Create a new record for the key in the batch file
			//
			// This is the only time we can't reuse a record and need
			// to re-add to the map (this is to keep a set of pending records
			// prior to grabbing the key lock).
			r := &record{batch: b.batch, key: next.key}
			start, err := b.writePut(next.key, value)
			if err != nil {
				return false, err
			}
			if len(value) >= minDiskValueSize {
				r.loc = start
				r.size = uint32(len(value))
			} else {
				r.value = value
				r.loc = -1
			}
			op := opPutLen(next.key, value)
			b.openWrites += op
			b.t.aliveBytes += op
			b.t.alive.Add(r)
		}
		return true, nil
	}

	// Open old batch for writing
	b.a.logger.Debug("continuing to build on old batch", zap.Uint64("old", oldestBatch), zap.Uint64("new", b.batch))
	b.movingPath = filepath.Join(b.a.baseDir, strconv.FormatUint(oldestBatch, 10))
	f, err := os.OpenFile(b.movingPath, os.O_WRONLY, 0666)
	if err != nil {
		return false, err
	}
	fi, err := f.Stat()
	if err != nil {
		return false, err
	}
	b.f = f
	b.writer = newWriter(f, fi.Size(), b.a.bufferSize)
	b.cursor = int64(previous.reader.Len())
	b.startingCursor = b.cursor
	b.t.uselessBytes = previous.uselessBytes
	b.t.aliveBytes = previous.aliveBytes
	if _, err := b.hasher.Write(previous.checksum[:]); err != nil {
		return false, err
	}
	b.t.alive = previous.alive
	if err := b.writeBatch(); err != nil {
		return false, err
	}
	opLen := opBatchLen()
	b.openWrites += opLen
	b.t.uselessBytes += opLen
	return true, nil
}

// TODO: make this a true abort as long as called before Prepare
//
// We must release this lock to shutdown properly
func (b *Batch) Abort() error {
	// Close in-progress file
	if err := b.f.Close(); err != nil {
		return err
	}

	// Cleanup aborted work
	if len(b.movingPath) == 0 {
		if err := os.Remove(b.path); err != nil {
			return err
		}
	} else {
		if err := os.Truncate(b.movingPath, b.startingCursor); err != nil {
			return err
		}
	}
	b.a.nextBatch--

	// Release lock held acquired during [recycle]
	b.a.commitLock.Unlock()
	b.a.logger.Debug("aborted batch", zap.Uint64("batch", b.batch))
	return nil
}

func (b *Batch) writePut(key string, value []byte) (int64, error) {
	// Check input
	if len(key) > int(consts.MaxUint16) {
		return -1, ErrKeyTooLong
	}

	// Write to disk
	errs := &wrappers.Errs{}
	b.buf = b.buf[:1]
	b.buf[0] = opPut
	errs.Add(b.writeBuffer(b.buf, true))
	b.buf = b.buf[:consts.Uint16Len]
	binary.BigEndian.PutUint16(b.buf, uint16(len(key)))
	errs.Add(b.writeBuffer(b.buf, true))
	errs.Add(b.writeBuffer(string2bytes(key), true))
	b.buf = b.buf[:consts.Uint32Len]
	binary.BigEndian.PutUint32(b.buf, uint32(len(value)))
	errs.Add(b.writeBuffer(b.buf, true))
	valueStart := b.cursor
	errs.Add(b.writeBuffer(value, true))
	return valueStart, errs.Err
}

func (b *Batch) writeBatch() error {
	// Write to disk
	errs := &wrappers.Errs{}
	b.buf = b.buf[:1]
	b.buf[0] = opBatch
	errs.Add(b.writeBuffer(b.buf, true))
	b.buf = b.buf[:consts.Uint64Len]
	binary.BigEndian.PutUint64(b.buf, b.batch)
	errs.Add(b.writeBuffer(b.buf, true))
	return errs.Err
}

func (b *Batch) writeChecksum() (ids.ID, error) {
	// Write to disk
	errs := &wrappers.Errs{}
	b.buf = b.buf[:1]
	b.buf[0] = opChecksum
	errs.Add(b.writeBuffer(b.buf, true))
	checksum := ids.ID(b.hasher.Sum(nil))
	errs.Add(b.writeBuffer(checksum[:], false))
	return checksum, errs.Err
}

// Prepare should be called right before we begin writing to the batch. As soon as
// this function is called, all reads to the database will be blocked.
//
// Prepare returns how many bytes were written to prepare the batch
// and whether we are rewriting an old batch file.
func (b *Batch) Prepare() (int64, bool) {
	b.a.keyLock.Lock()

	if len(b.movingPath) == 0 {
		// Iterate over [alive] and update records for [keys] that were recycled
		iter := b.t.alive.Iterator()
		for next := iter.Next(); next != nil; next = iter.Next() {
			b.a.keys[next.key] = next
			b.a.logger.Debug("updating key to new record", zap.String("key", next.key), zap.Uint64("batch", next.batch))
		}
	} else {
		// Migrate all alive keys to the new batch lookup.
		iter := b.t.alive.Iterator()
		for next := iter.Next(); next != nil; next = iter.Next() {
			next.batch = b.batch
			b.a.logger.Debug("migrating key to new batch", zap.String("key", next.key), zap.Uint64("prev", next.batch), zap.Uint64("new", b.batch))
		}
	}

	// Setup tracker for reference by this batch
	//
	// We wait to do this until here because we need to hold
	// [keyLock] to ensure that [b.a.batches] is not referenced
	// at the same time.
	b.a.batches[b.batch] = b.t
	return b.openWrites, b.pruneableBatch != nil && len(b.movingPath) == 0
}

func (b *Batch) Put(_ context.Context, key string, value []byte) error {
	start, err := b.writePut(key, value)
	if err != nil {
		return err
	}
	past, ok := b.a.keys[key]
	if ok {
		b.a.batches[past.batch].Remove(past)
		past.batch = b.batch

		// We avoid updating [keys] when just updating the value of a [record]
	} else {
		past = &record{batch: b.batch, key: key}
		b.a.keys[key] = past
	}
	if len(value) >= minDiskValueSize {
		past.value = nil
		past.loc = start
		past.size = uint32(len(value))
	} else {
		past.value = value
		past.loc = -1
		past.size = 0
	}
	b.t.alive.Add(past)
	b.t.aliveBytes += opPutLen(key, value)
	return nil
}

func (b *Batch) Delete(_ context.Context, key string) error {
	// Check input
	if len(key) > int(consts.MaxUint16) {
		return ErrKeyTooLong
	}

	// Check if key even exists
	past, ok := b.a.keys[key]
	if !ok {
		b.a.logger.Debug("attempted to delete non-existent key", zap.String("key", key))
		return nil
	}

	// Write to disk
	errs := &wrappers.Errs{}
	b.buf = b.buf[:1]
	b.buf[0] = opDelete
	errs.Add(b.writeBuffer(b.buf, true))
	b.buf = b.buf[:consts.Uint16Len]
	binary.BigEndian.PutUint16(b.buf, uint16(len(key)))
	errs.Add(b.writeBuffer(b.buf, true))
	errs.Add(b.writeBuffer(string2bytes(key), true))
	if errs.Err != nil {
		return errs.Err
	}

	// Account for useless bytes
	//
	// We check to see if the past batch is less
	// than the current batch because we may have just recycled
	// this key and it is already in [alive].
	b.a.batches[past.batch].Remove(past)
	delete(b.a.keys, key)
	b.t.uselessBytes += opDeleteLen(key)
	return nil
}

func (b *Batch) Get(ctx context.Context, key string) ([]byte, error) {
	return b.a.Get(ctx, key)
}

// Write failure is not expected. If an error is returned,
// it should be treated as fatal.
func (b *Batch) Write() (ids.ID, error) {
	defer func() {
		b.a.keyLock.Unlock()
		b.a.commitLock.Unlock()
	}()

	// Add batch and checksum to file (allows for crash recovery on restart in the case that a file is partially written)
	checksum, err := b.writeChecksum()
	if err != nil {
		return ids.Empty, err
	}
	// We always consider previous checksums useless in the case
	// that we are reusing a file.
	b.t.uselessBytes += opChecksumLen()

	// Flush all unwritten data to disk
	if err := b.writer.Flush(); err != nil {
		return ids.Empty, fmt.Errorf("%w: could not flush file", err)
	}

	// Close file now that we don't need to write to it anymore
	//
	// Note: we don't require the file to be fsync'd here and assume
	// we can recover the current state on restart.
	if err := b.f.Close(); err != nil {
		return ids.Empty, fmt.Errorf("%w: could not close file", err)
	}

	// Handle old batch cleanup
	if b.pruneableBatch != nil {
		preparedBatch := *b.pruneableBatch
		preparedReader := b.a.batches[preparedBatch]
		if err := preparedReader.reader.Close(); err != nil {
			return ids.Empty, fmt.Errorf("%w: could not close old batch", err)
		}
		if len(b.movingPath) == 0 {
			preparedPath := filepath.Join(b.a.baseDir, strconv.FormatUint(preparedBatch, 10))
			if err := os.Remove(preparedPath); err != nil {
				return ids.Empty, fmt.Errorf("%w: could not remove old batch", err)
			}
		} else {
			if err := os.Rename(b.movingPath, b.path); err != nil {
				return ids.Empty, fmt.Errorf("%w: could not rename file", err)
			}
		}
		delete(b.a.batches, preparedBatch)
		oldestBatch := preparedBatch + 1
		b.a.oldestBatch = &oldestBatch
	}

	// Open file for mmap before keys become acessible
	reader, err := mmap.Open(b.path)
	if err != nil {
		// Should never happen
		return ids.Empty, fmt.Errorf("%w: could not mmap new batch", err)
	}

	// Register the batch for reading
	b.t.reader = reader
	b.t.checksum = checksum

	// Set oldest batch if haven't done yet
	if b.a.oldestBatch == nil {
		b.a.oldestBatch = &b.batch
	}
	return checksum, nil
}

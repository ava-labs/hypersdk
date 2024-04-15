package vilmo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/state"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
)

var _ state.Immutable = (*Vilmo)(nil)

type Vilmo struct {
	logger     logging.Logger
	baseDir    string
	bufferSize int
	historyLen int

	commitLock sync.RWMutex
	nextBatch  uint64
	batches    map[uint64]*log

	keyLock sync.RWMutex
	keys    map[string]*record
}

// New returns a new Vilmo instance and the ID of the last committed file.
func New(
	logger logging.Logger,
	baseDir string,
	initialSize int,
	batchSize int,
	bufferSize int,
	historyLen int, // should not be changed
) (*Vilmo, ids.ID, error) {
	// Iterate over files in directory and put into sorted order
	start := time.Now()
	files := []uint64{}
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if baseDir == path {
			// Don't attempt to load self
			return nil
		}
		if info.IsDir() {
			// Skip anything unexpected
			logger.Warn("found unexpected directory", zap.String("path", path))
			return nil
		}
		file, err := strconv.ParseUint(info.Name(), 10, 64)
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	})
	if err != nil {
		return nil, ids.Empty, err
	}

	// Load log files from disk
	//
	// During this process, we attempt to recover any corrupt files
	// on-disk and ensure that we don't have any gaps. We also ensure we don't
	// end up with too many files.
	batchOps := map[uint64][]any{}
	batches := make(map[uint64]*log, len(files))
	for _, file := range files {
		path := filepath.Join(baseDir, strconv.FormatUint(file, 10))
		l, allOps, err := load(logger, file, path)
		if err != nil {
			logger.Warn("could not load log", zap.String("path", path), zap.Error(err))
			return nil, ids.Empty, err
		}
		if l == nil {
			// This means the file was empty and is now deleted
			continue
		}
		for batch, ops := range allOps {
			if _, ok := batchOps[batch]; ok {
				logger.Warn("found duplicate batch", zap.Uint64("batch", batch), zap.String("current", path))
				return nil, ids.Empty, errors.New("duplicate batch")
			}
			batchOps[batch] = ops
		}
		batches[l.batch] = l
	}
	if len(batches) > historyLen+1 {
		logger.Warn("found too many logs", zap.Int("count", len(batches)))
		return nil, ids.Empty, errors.New("too many logs")
	}

	// Build current state from all log files
	var (
		checksum          ids.ID
		keys              = make(map[string]*record, initialSize)
		replayableBatches = maps.Keys(batchOps)
	)
	slices.Sort(replayableBatches)
	for _, batch := range replayableBatches {
		// There may be gaps between batches depending on which files were rewritten,
		// that's ok.
		for _, op := range batchOps[batch] {
			switch o := op.(type) {
			case nil:
				// This happens when a put operation is nullified
				continue
			case *putOp:
				past, ok := keys[o.key]
				if ok {
					past.log.Remove(past, o.record.log != past.log)
				}
				record := o.record
				keys[o.key] = record
				o.record.log.Add(record)
			case *deleteOp:
				past, ok := keys[o.key]
				if !ok {
					continue
				}
				past.log.Remove(past, o.log != past.log)
			case ids.ID:
				checksum = o
			default:
				logger.Warn("found invalid operation", zap.Uint64("batch", batch), zap.Any("op", op))
				return nil, ids.Empty, errors.New("invalid operation")
			}
		}
	}

	// Instantiate DB
	adb := &Vilmo{
		logger:     logger,
		baseDir:    baseDir,
		bufferSize: bufferSize,
		historyLen: historyLen,

		keys:    keys,
		batches: batches,
	}
	if len(replayableBatches) > 0 {
		adb.nextBatch = replayableBatches[len(replayableBatches)-1] + 1
	}
	logger.Info(
		"loaded batches",
		zap.Int("count", len(adb.batches)),
		zap.Int("keys", len(adb.keys)),
		zap.Uint64("next batch", adb.nextBatch),
		zap.Stringer("last checksum", checksum),
		zap.Duration("duration", time.Since(start)),
	)
	return adb, checksum, nil
}

func (a *Vilmo) get(key string) ([]byte, error) {
	entry, ok := a.keys[key]
	if !ok {
		return nil, database.ErrNotFound
	}
	if entry.cached {
		return slices.Clone(entry.value), nil
	}
	value := make([]byte, entry.size)
	_, err := entry.log.reader.ReadAt(value, entry.ValueLoc())
	return value, err
}

func (a *Vilmo) Get(_ context.Context, key string) ([]byte, error) {
	a.keyLock.RLock()
	defer a.keyLock.RUnlock()

	return a.get(key)
}

func (a *Vilmo) Gets(_ context.Context, keys []string) ([][]byte, []error) {
	a.keyLock.RLock()
	defer a.keyLock.RUnlock()

	values := make([][]byte, len(keys))
	errors := make([]error, len(keys))
	for i, key := range keys {
		values[i], errors[i] = a.get(key)
	}
	return values, errors
}

func (a *Vilmo) Close() error {
	a.commitLock.Lock()
	defer a.commitLock.Unlock()

	for _, file := range a.batches {
		if err := file.reader.Close(); err != nil {
			return err
		}
	}
	a.logger.Info(
		"closing vilmo",
		zap.Int("keys", len(a.keys)),
		zap.Uint64("next batch", a.nextBatch),
		zap.Uint64("logs", uint64(len(a.batches))),
	)
	return nil
}

func (a *Vilmo) Usage() (int, int64 /* alive bytes */, int64 /* useless bytes */) {
	a.commitLock.RLock()
	defer a.commitLock.RUnlock()

	var (
		aliveBytes   int64
		uselessBytes int64
	)
	for n, batch := range a.batches {
		aliveBytes += batch.aliveBytes

		// This includes discarded data that may be deleted (may be a little inaccurate)
		uselessBytes += batch.uselessBytes
		a.logger.Debug(
			"batch usage",
			zap.Uint64("batch", n),
			zap.Int64("alive", batch.aliveBytes),
			zap.Int64("useless", batch.uselessBytes),
			zap.Int64("total", batch.aliveBytes+batch.uselessBytes),
		)
	}
	return len(a.keys), aliveBytes, uselessBytes
}

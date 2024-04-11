package appenddb

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/linked"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/smap"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	defaultInitialSize = 10_000_000
	defaultBufferSize  = 64 * units.KiB
	defaultHistoryLen  = 100
)

// reuseKeys is a value [0,itemsPerBatch] that determines how many keys are reused across batches
func randomKeyValues(batches int, itemsPerBatch int, keySize int, valueSize int, reuseKeys int) ([][][]byte, [][][]byte) {
	keys := make([][][]byte, batches)
	values := make([][][]byte, batches)
	for i := 0; i < batches; i++ {
		keys[i] = make([][]byte, itemsPerBatch)
		values[i] = make([][]byte, itemsPerBatch)
		for j := 0; j < itemsPerBatch; j++ {
			// Create key
			if i == 0 || j >= reuseKeys {
				k := make([]byte, keySize)
				rand.Read(k)
				keys[i][j] = k
			} else {
				keys[i][j] = keys[0][j]
			}

			// Create value
			v := make([]byte, valueSize)
			rand.Read(v)
			values[i][j] = v
		}
	}
	return keys, values
}

func TestAppendDB(t *testing.T) {
	// Prepare
	require := require.New(t)
	ctx := context.TODO()
	baseDir := t.TempDir()
	logger := logging.NewLogger(
		"appenddb",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stdout,
			logging.Colors.ConsoleEncoder(),
		),
	)
	logger.Info("created directory", zap.String("path", baseDir))

	// Create
	db, last, err := New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, defaultHistoryLen)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Put
	b, err := db.NewBatch()
	require.NoError(err)
	require.Zero(b.Prepare())
	require.NoError(b.Put(ctx, "hello", []byte("world")))
	batch, err := b.Write()
	require.NoError(err)
	require.NotEqual(ids.Empty, batch)

	// Get
	v, err := db.Get(ctx, "hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)

	// Restart
	keys, aliveBytes, uselessBytes := db.Usage()
	require.NoError(db.Close())
	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, defaultHistoryLen)
	require.NoError(err)
	require.Equal(batch, last)
	keys2, aliveBytes2, uselessBytes2 := db.Usage()
	require.Equal(keys, keys2)
	require.Equal(aliveBytes, aliveBytes2)
	require.Equal(uselessBytes, uselessBytes2)

	// Get
	v, err = db.Get(ctx, "hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)

	// Move file
	require.NoError(os.Rename(filepath.Join(baseDir, "0"), filepath.Join(baseDir, "100")))
	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, defaultHistoryLen)
	require.ErrorIs(err, ErrCorrupt)
	require.NoError(os.Rename(filepath.Join(baseDir, "100"), filepath.Join(baseDir, "0")))

	// Ensure restart still works
	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, defaultHistoryLen)
	require.NoError(err)
	require.Equal(batch, last)
	require.NoError(db.Close())

	// Modify file
	f, err := os.OpenFile(filepath.Join(baseDir, "0"), os.O_RDWR, os.ModeAppend)
	require.NoError(err)
	_, err = f.WriteString("corrupt")
	require.NoError(err)
	require.NoError(f.Close())

	// Restart
	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, defaultHistoryLen)
	require.ErrorIs(err, ErrCorrupt)
}

func TestAppendDBAbort(t *testing.T) {
	// Prepare
	require := require.New(t)
	ctx := context.TODO()
	baseDir := t.TempDir()
	logger := logging.NewLogger(
		"appenddb",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stdout,
			logging.Colors.ConsoleEncoder(),
		),
	)
	logger.Info("created directory", zap.String("path", baseDir))

	// Create
	db, last, err := New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 1)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Insert key (add enough items such that nullifiers will be written rather than rewriting file)
	b, err := db.NewBatch()
	require.NoError(err)
	require.Zero(b.Prepare())
	require.NoError(b.Put(ctx, "hello", []byte("world")))
	require.NoError(b.Put(ctx, "hello2", []byte("world2")))
	require.NoError(b.Put(ctx, "hello3", []byte("world3")))
	_, err = b.Write()
	require.NoError(err)

	// Create a batch gap
	b, err = db.NewBatch()
	require.NoError(err)
	openBytes, rewrite := b.Prepare()
	require.Equal(int64(0), openBytes)
	require.False(rewrite)
	require.NoError(b.Put(ctx, "hello", []byte("world10")))
	checksum, err := b.Write()
	require.NoError(err)
	keys, alive, useless := db.Usage()

	// Create new batch then abort
	b, err = db.NewBatch()
	require.NoError(err)
	require.NoError(b.Abort())
	require.NoError(db.Close())

	// Reload database
	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 1)
	require.NoError(err)
	require.Equal(checksum, last)
	keys2, alive2, useless2 := db.Usage()
	require.Equal(keys, keys2)
	require.Equal(alive, alive2)
	require.Equal(useless, useless2)

	// Write batch
	b, err = db.NewBatch()
	require.NoError(err)
	openBytes, rewrite = b.Prepare()
	require.Equal(int64(8), openBytes)
	require.False(rewrite)
	require.NoError(b.Put(ctx, "hello", []byte("world11")))
	require.NoError(b.Delete(ctx, "hello2"))
	checksum, err = b.Write()
	require.NoError(err)
	keys, alive, useless = db.Usage()
	require.NoError(db.Close())

	// Reload database and ensure nullifiers were written
	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 1)
	require.NoError(err)
	require.Equal(checksum, last)
	keys2, alive2, useless2 = db.Usage()
	require.Equal(keys, keys2)
	require.Equal(alive, alive2)
	require.Equal(useless, useless2)

	// Ensure data is correct
	v, err := db.Get(ctx, "hello")
	require.NoError(err)
	require.Equal([]byte("world11"), v)
	_, err = db.Get(ctx, "hello2")
	require.ErrorIs(err, database.ErrNotFound)
	v, err = db.Get(ctx, "hello3")
	require.NoError(err)
	require.Equal([]byte("world3"), v)
	require.NoError(db.Close())
}

func TestAppendDBReinsertHistory(t *testing.T) {
	// Prepare
	require := require.New(t)
	ctx := context.TODO()
	baseDir := t.TempDir()
	logger := logging.NewLogger(
		"appenddb",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stdout,
			logging.Colors.ConsoleEncoder(),
		),
	)
	logger.Info("created directory", zap.String("path", baseDir))

	// Create
	db, last, err := New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 1)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Insert key
	b, err := db.NewBatch()
	require.NoError(err)
	require.Zero(b.Prepare())
	require.NoError(b.Put(ctx, "hello", []byte("world")))
	_, err = b.Write()
	require.NoError(err)

	// Create a batch gap
	b, err = db.NewBatch()
	require.NoError(err)
	openBytes, rewrite := b.Prepare()
	require.Equal(int64(0), openBytes)
	require.False(rewrite)
	require.NoError(b.Put(ctx, "world", []byte("hello")))
	_, err = b.Write()
	require.NoError(err)
	require.Zero(len(db.batches[1].pendingNullify))

	// Modify recycled key
	b, err = db.NewBatch()
	require.NoError(err)
	openBytes, rewrite = b.Prepare()
	require.Equal(int64(0), openBytes) // no changes since last opened
	require.False(rewrite)
	require.NoError(b.Put(ctx, "hello", []byte("world2")))
	_, err = b.Write()
	require.NoError(err)
	require.Zero(len(db.batches[2].pendingNullify))

	// Delete recycled key
	b, err = db.NewBatch()
	require.NoError(err)
	openBytes, rewrite = b.Prepare()
	require.Equal(int64(0), openBytes)
	require.False(rewrite)
	require.NoError(b.Delete(ctx, "world"))
	checksum, err := b.Write()
	require.NoError(err)
	keys, alive, useless := db.Usage()
	require.Zero(len(db.batches[3].pendingNullify))

	// Restart and ensure data is correct
	require.NoError(db.Close())
	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 1)
	require.NoError(err)
	require.Equal(checksum, last)
	v, err := db.Get(ctx, "hello")
	require.NoError(err)
	require.Equal([]byte("world2"), v)
	_, err = db.Get(ctx, "world")
	require.ErrorIs(err, database.ErrNotFound)
	keys2, alive2, useless2 := db.Usage()
	require.Equal(keys, keys2)
	require.Equal(alive, alive2)
	require.Equal(useless, useless2)
}

func TestAppendDBClearNullifyOnNew(t *testing.T) {
	// Prepare
	require := require.New(t)
	ctx := context.TODO()
	baseDir := t.TempDir()
	logger := logging.NewLogger(
		"appenddb",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stdout,
			logging.Colors.ConsoleEncoder(),
		),
	)
	logger.Info("created directory", zap.String("path", baseDir))

	// Create
	db, last, err := New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 1)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Insert key
	b, err := db.NewBatch()
	require.NoError(err)
	require.Zero(b.Prepare())
	require.NoError(b.Put(ctx, "hello", []byte("world")))
	require.NoError(b.Put(ctx, "hello1", []byte("world")))
	require.NoError(b.Put(ctx, "hello2", []byte("world")))
	require.NoError(b.Put(ctx, "hello3", []byte("world")))
	_, err = b.Write()
	require.NoError(err)

	// Insert a batch gap (add nullifiers)
	b, err = db.NewBatch()
	require.NoError(err)
	require.Zero(b.Prepare())
	require.NoError(b.Delete(ctx, "hello"))
	require.NoError(b.Delete(ctx, "hello1"))
	require.NoError(b.Delete(ctx, "hello2"))
	require.NoError(b.Delete(ctx, "hello3"))
	_, err = b.Write()
	require.NoError(err)

	// Rewrite batch
	b, err = db.NewBatch()
	require.NoError(err)
	initBytes, rewrite := b.Prepare()
	require.True(rewrite)
	require.Equal(int64(0), initBytes)
	_, err = b.Write()
	require.NoError(err)
	require.Zero(len(db.batches[2].pendingNullify))
	require.NoError(db.Close())
}

func TestAppendDBPrune(t *testing.T) {
	// Prepare
	require := require.New(t)
	ctx := context.TODO()
	baseDir := t.TempDir()
	logger := logging.NewLogger(
		"appenddb",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stdout,
			logging.Colors.ConsoleEncoder(),
		),
	)
	logger.Info("created directory", zap.String("path", baseDir))

	// Create
	db, last, err := New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 10)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Insert 100 batches
	var lastBatch ids.ID
	for i := 0; i < 100; i++ {
		b, err := db.NewBatch()
		require.NoError(err)
		b.Prepare()
		switch {
		case i == 0:
			// Never modify again
			require.NoError(b.Put(ctx, "hello", []byte("world")))
		case i < 99:
			for j := 0; j < 10; j++ {
				require.NoError(b.Put(ctx, strconv.Itoa(j), []byte(strconv.Itoa(i))))
			}
		default:
			require.NoError(b.Delete(ctx, strconv.Itoa(0)))
		}
		lastBatch, err = b.Write()
		require.NoError(err)
		require.NotEqual(ids.Empty, lastBatch)
	}

	// Ensure data is correct
	v, err := db.Get(ctx, "hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)
	for i := 0; i < 10; i++ {
		v, err = db.Get(ctx, strconv.Itoa(i))
		if i == 0 {
			require.ErrorIs(err, database.ErrNotFound)
		} else {
			require.NoError(err)
			require.Equal([]byte("98"), v)
		}
	}

	// Ensure files were pruned
	keys, alive, useless := db.Usage()
	require.NoError(db.Close())
	files, err := os.ReadDir(baseDir)
	require.NoError(err)
	require.Len(files, 11) // 10 historical batches

	// Restart
	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 10)
	require.NoError(err)
	require.Equal(lastBatch, last)
	keys2, alive2, useless2 := db.Usage()
	require.Equal(keys, keys2)
	require.Equal(alive, alive2)
	require.Equal(useless, useless2)

	// Ensure data is correct after restart
	v, err = db.Get(ctx, "hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)
	for i := 0; i < 10; i++ {
		v, err = db.Get(ctx, strconv.Itoa(i))
		if i == 0 {
			require.ErrorIs(err, database.ErrNotFound)
		} else {
			require.NoError(err)
			require.Equal([]byte("98"), v)
		}
	}

	// Write to new batches
	for i := 0; i < 10; i++ {
		b, err := db.NewBatch()
		require.NoError(err)
		b.Prepare()
		for j := 0; j < 10; j++ {
			if i == 5 {
				require.NoError(b.Delete(ctx, strconv.Itoa(j)))
			} else {
				require.NoError(b.Put(ctx, strconv.Itoa(j), []byte(strconv.Itoa(i))))
			}
		}
		lastBatch, err = b.Write()
		require.NoError(err)
		require.NotEqual(ids.Empty, lastBatch)
	}
	keys, alive, useless = db.Usage()
	require.NoError(db.Close())
	files, err = os.ReadDir(baseDir)
	require.NoError(err)
	require.Len(files, 11) // 10 historical batches

	// Read from new batches
	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 10)
	require.NoError(err)
	require.Equal(lastBatch, last)
	keys2, alive2, useless2 = db.Usage()
	require.Equal(keys, keys2)
	require.Equal(alive, alive2)
	require.Equal(useless, useless2)
	for i := 0; i < 10; i++ {
		v, err = db.Get(ctx, strconv.Itoa(i))
		require.NoError(err)
		require.Equal([]byte(strconv.Itoa(9)), v)
	}
	v, err = db.Get(ctx, "hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)
	require.NoError(db.Close())
}

func TestAppendDBLarge(t *testing.T) {
	for _, valueSize := range []int{32, minDiskValueSize * 2} { // ensure mem and mmap work
		t.Run(fmt.Sprintf("valueSize=%d", valueSize), func(t *testing.T) {
			// Prepare
			require := require.New(t)
			ctx := context.TODO()
			baseDir := t.TempDir()
			logger := logging.NewLogger(
				"appenddb",
				logging.NewWrappedCore(
					logging.Debug,
					os.Stdout,
					logging.Colors.ConsoleEncoder(),
				),
			)
			logger.Info("created directory", zap.String("path", baseDir))

			// Create
			batchSize := 10_000
			db, last, err := New(logger, baseDir, defaultInitialSize, batchSize, defaultBufferSize, 5)
			require.NoError(err)
			require.Equal(ids.Empty, last)

			// Write 1M unique keys in 10 batches
			batches := 10
			keys, values := randomKeyValues(batches, batchSize, 32, 32, 0)
			checksums := make([]ids.ID, batches)
			for i := 0; i < batches; i++ {
				b, err := db.NewBatch()
				require.NoError(err)
				openBytes, rewrite := b.Prepare()
				if i <= 5 {
					require.Zero(openBytes)
					require.False(rewrite)
				} else {
					require.Equal(int64(0), openBytes) // all no keys, no nullifiers
					require.False(rewrite)
				}
				for j := 0; j < batchSize; j++ {
					require.NoError(b.Put(ctx, string(keys[i][j]), values[i][j]))
				}
				checksum, err := b.Write()
				require.NoError(err)

				// Ensure data is correct
				for j := 0; j < batchSize; j++ {
					v, err := db.Get(ctx, string(keys[i][j]))
					require.NoError(err)
					require.Equal(values[i][j], v)
				}
				checksums[i] = checksum
			}

			// Restart
			aliveKeys, aliveBytes, uselessBytes := db.Usage()
			require.NoError(db.Close())
			db, last, err = New(logger, baseDir, defaultInitialSize, batchSize, defaultBufferSize, 5)
			require.NoError(err)
			require.Equal(checksums[batches-1], last)
			aliveKeys2, aliveBytes2, uselessBytes2 := db.Usage()
			require.Equal(aliveKeys, aliveKeys2)
			require.Equal(aliveBytes, aliveBytes2)
			require.Equal(uselessBytes, uselessBytes2)

			// Ensure data is correct after restart
			for i := 0; i < batchSize; i++ {
				v, err := db.Get(ctx, string(keys[9][i]))
				require.NoError(err)
				require.Equal(values[9][i], v)
			}
			require.NoError(db.Close())

			// Create another database and ensure checksums match
			db2, last, err := New(logger, t.TempDir(), defaultInitialSize, batchSize, defaultBufferSize, 5)
			require.NoError(err)
			require.Equal(ids.Empty, last)
			for i := 0; i < batches; i++ {
				b, err := db2.NewBatch()
				require.NoError(err)
				b.Prepare()
				for j := 0; j < batchSize; j++ {
					require.NoError(b.Put(ctx, string(keys[i][j]), values[i][j]))
				}
				checksum, err := b.Write()
				require.NoError(err)
				require.Equal(checksums[i], checksum)
			}
		})
	}
}

func BenchmarkAppendDB(b *testing.B) {
	ctx := context.TODO()
	batches := 10
	for _, batchSize := range []int{25_000, 50_000, 100_000, 500_000, 1_000_000} {
		for _, reuse := range []int{0, batchSize / 4, batchSize / 3, batchSize / 2, batchSize} {
			for _, historyLen := range []int{1, 5, 10} {
				for _, bufferSize := range []int{2 * units.KiB, 4 * units.KiB, defaultBufferSize, 4 * defaultBufferSize} {
					keys, values := randomKeyValues(batches, batchSize, 32, 32, reuse)
					b.Run(fmt.Sprintf("keys=%d reuse=%d history=%d buffer=%d", batchSize, reuse, historyLen, bufferSize), func(b *testing.B) {
						for i := 0; i < b.N; i++ {
							db, _, err := New(logging.NoLog{}, b.TempDir(), defaultInitialSize, batchSize, bufferSize, historyLen)
							if err != nil {
								b.Error(err)
							}
							for j := 0; j < batches; j++ {
								batch, err := db.NewBatch()
								if err != nil {
									b.Error(err)
								}
								batch.Prepare()
								for k := 0; k < batchSize; k++ {
									if err := batch.Put(ctx, string(keys[j][k]), values[j][k]); err != nil {
										b.Error(err)
									}
								}
								if _, err = batch.Write(); err != nil {
									b.Error(err)
								}
							}
							if err := db.Close(); err != nil {
								b.Error(err)
							}
						}
					})
				}
			}
		}
	}
}

func BenchmarkPebbleDB(b *testing.B) {
	batches := 10
	for _, batchSize := range []int{25_000, 50_000, 100_000, 500_000, 1_000_000} {
		for _, reuse := range []int{0, batchSize / 4, batchSize / 3, batchSize / 2, batchSize} {
			keys, values := randomKeyValues(batches, batchSize, 32, 32, reuse)
			b.Run(fmt.Sprintf("keys=%d reuse=%d", batchSize, reuse), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					db, _, err := pebble.New(b.TempDir(), pebble.NewDefaultConfig())
					if err != nil {
						b.Error(err)
					}
					for j := 0; j < batches; j++ {
						batch := db.NewBatch()
						for k := 0; k < batchSize; k++ {
							if err := batch.Put(keys[j][k], values[j][k]); err != nil {
								b.Error(err)
							}
						}
						if err := batch.Write(); err != nil {
							b.Error(err)
						}
					}
					if err := db.Close(); err != nil {
						b.Error(err)
					}
				}
			})
		}
	}
}

func simpleRandomKeyValues(items int, size int) ([]string, [][]byte) {
	keys := make([]string, items)
	values := make([][]byte, items)
	for i := 0; i < items; i++ {
		k := make([]byte, size)
		rand.Read(k)
		keys[i] = string(k)
		v := make([]byte, size)
		rand.Read(v)
		values[i] = v
	}
	return keys, values
}

type hasmapIterator struct {
	hm *linked.Hashmap[string, []byte]
}

func (hi *hasmapIterator) Iterate(f func(k string, v []byte) error) error {
	iter := hi.hm.NewIterator()
	for iter.Next() {
		if err := f(iter.Key(), iter.Value()); err != nil {
			return err
		}
	}
	return nil
}

func BenchmarkWriter(b *testing.B) {
	var (
		items          = 100_000
		pkeys, pvalues = simpleRandomKeyValues(items, 32)
	)

	b.Run("direct", func(b *testing.B) {
		require := require.New(b)
		for i := 0; i < b.N; i++ {
			dir := b.TempDir()
			f, err := os.Create(filepath.Join(dir, "file"))
			require.NoError(err)
			for j := 0; j < items; j++ {
				_, err := f.Write([]byte(pkeys[j]))
				require.NoError(err)
				_, err = f.Write(pvalues[j])
				require.NoError(err)
			}
			require.NoError(f.Close())
		}
	})

	b.Run("bufio", func(b *testing.B) {
		require := require.New(b)
		for i := 0; i < b.N; i++ {
			dir := b.TempDir()
			f, err := os.Create(filepath.Join(dir, "file"))
			require.NoError(err)
			w := bufio.NewWriterSize(f, defaultBufferSize)
			for j := 0; j < items; j++ {
				_, err := w.Write([]byte(pkeys[j]))
				require.NoError(err)
				_, err = w.Write(pvalues[j])
				require.NoError(err)
			}
			require.NoError(w.Flush())
			require.NoError(f.Close())
		}
	})

	b.Run("no-block writer", func(b *testing.B) {
		require := require.New(b)
		for i := 0; i < b.N; i++ {
			dir := b.TempDir()
			f, err := os.Create(filepath.Join(dir, "file"))
			require.NoError(err)
			w := newWriter(f, 0, defaultBufferSize)
			for j := 0; j < items; j++ {
				w.Write([]byte(pkeys[j]))
				w.Write(pvalues[j])
			}
			require.NoError(w.Flush())
			require.NoError(f.Close())
		}
	})

	b.Run("batch", func(b *testing.B) {
		require := require.New(b)
		db, last, err := New(logging.NoLog{}, b.TempDir(), defaultInitialSize, 100_000, defaultBufferSize, 15)
		require.NoError(err)
		require.Equal(ids.Empty, last)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b, err := db.NewBatch()
			require.NoError(err)
			b.Prepare()
			for j := 0; j < items; j++ {
				require.NoError(b.Put(context.TODO(), pkeys[j], pvalues[j]))
			}
			_, err = b.Write()
			require.NoError(err)
		}
		require.NoError(db.Close())
	})

	hm := linked.NewHashmap[string, []byte]()
	for i := 0; i < items; i++ {
		hm.Put(pkeys[i], pvalues[i])
	}
	b.Run("hashmap", func(b *testing.B) {
		require := require.New(b)
		db, last, err := New(logging.NoLog{}, b.TempDir(), defaultInitialSize, 100_000, defaultBufferSize, 15)
		require.NoError(err)
		require.Equal(ids.Empty, last)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b, err := db.NewBatch()
			require.NoError(err)
			b.Prepare()
			iter := hm.NewIterator()
			for iter.Next() {
				require.NoError(b.Put(context.TODO(), iter.Key(), iter.Value()))
			}
			_, err = b.Write()
			require.NoError(err)
		}
		require.NoError(db.Close())
	})

	hm2 := linked.NewHashmap[string, []byte]()
	for i := 0; i < items; i++ {
		hm2.Put(pkeys[i], pvalues[i])
	}
	hmi := &hasmapIterator{hm: hm2}
	b.Run("iterate", func(b *testing.B) {
		require := require.New(b)
		db, last, err := New(logging.NoLog{}, b.TempDir(), defaultInitialSize, 100_000, defaultBufferSize, 15)
		require.NoError(err)
		require.Equal(ids.Empty, last)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b, err := db.NewBatch()
			require.NoError(err)
			b.Prepare()
			require.NoError(hmi.Iterate(func(k string, v []byte) error {
				return b.Put(context.TODO(), k, v)
			}))
			_, err = b.Write()
			require.NoError(err)
		}
		require.NoError(db.Close())
	})

	smap := smap.New[[]byte](100_000)
	for i, key := range pkeys {
		smap.Put(key, pvalues[i])
	}
	b.Run("smap", func(b *testing.B) {
		require := require.New(b)
		db, last, err := New(logging.NoLog{}, b.TempDir(), defaultInitialSize, 100_000, defaultBufferSize, 15)
		require.NoError(err)
		require.Equal(ids.Empty, last)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b, err := db.NewBatch()
			require.NoError(err)
			b.Prepare()
			for _, key := range pkeys {
				value, _ := smap.Get(key)
				require.NoError(b.Put(context.TODO(), key, value))
			}
			_, err = b.Write()
			require.NoError(err)
		}
		require.NoError(db.Close())
	})

	ts := tstate.New(100_000 * 2)
	for i := 0; i < 10; i++ {
		ts.PrepareChunk(i, 100_000) // purposely add gaps
		for j := 0; j < 10_000; j++ {
			tsv := ts.NewWriteView(i, 10_000+j) // purposely add gap
			tsv.Put(context.TODO(), pkeys[i*10_000+j], pvalues[i*10_000+j])
			tsv.Put(context.TODO(), "blah", []byte("blah")) // ensure multiple values
			tsv.Commit()
		}
	}
	b.Run("tstate", func(b *testing.B) {
		require := require.New(b)
		db, last, err := New(logging.NoLog{}, b.TempDir(), defaultInitialSize, 100_000, defaultBufferSize, 15)
		require.NoError(err)
		require.Equal(ids.Empty, last)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b, err := db.NewBatch()
			require.NoError(err)
			b.Prepare()
			require.NoError(ts.Iterate(func(k string, v maybe.Maybe[[]byte]) error {
				if v.IsNothing() {
					return b.Delete(context.TODO(), k)
				} else {
					return b.Put(context.TODO(), k, v.Value())
				}
			}))
			_, err = b.Write()
			require.NoError(err)
		}
		require.NoError(db.Close())
	})
}

func BenchmarkMapUpdate(b *testing.B) {
	m := make(map[string][]byte, 10_000_000)
	pkeys, pvalues := simpleRandomKeyValues(10_000_000, 32)
	for i := 0; i < 10_000_000; i++ {
		m[pkeys[i]] = pvalues[i]
	}
	pkeysu, pvaluesu := simpleRandomKeyValues(100_000, 32)
	b.ResetTimer()
	for j := 0; j < 100_000; j++ {
		m[pkeysu[j]] = pvaluesu[j]
	}
}

func BenchmarkBatchRewrite(b *testing.B) {
	require := require.New(b)
	keys, values := randomKeyValues(30, 100_000, 32, 32, 100_000)
	db, last, err := New(logging.NoLog{}, b.TempDir(), defaultInitialSize, 100_000, defaultBufferSize, 15)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	b.Run("initial", func(b *testing.B) {
		batch, err := db.NewBatch()
		require.NoError(err)
		batch.Prepare()
		for i := 0; i < 100_000; i++ {
			require.NoError(batch.Put(context.TODO(), string(keys[0][i]), values[0][i]))
		}
		_, err = batch.Write()
		require.NoError(err)
	})

	for j := 1; j < 30; j++ {
		b.Run(fmt.Sprintf("rewrite=%d", j), func(b *testing.B) {
			batch, err := db.NewBatch()
			require.NoError(err)
			batch.Prepare()
			for i := 0; i < 100_000; i++ {
				require.NoError(batch.Put(context.TODO(), string(keys[j][i]), values[j][i]))
			}
			_, err = batch.Write()
			require.NoError(err)
		})

		require.NoError(db.Close())
	}
}

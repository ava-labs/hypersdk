package appenddb

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	defaultInitialSize = 10_000_000
	defaultBufferSize  = 64 * units.KiB
	defaultHistoryLen  = 100
)

func randomKeyValues(batches int, itemsPerBatch int, keySize int, valueSize int, reuseKeys bool) ([][][]byte, [][][]byte) {
	keys := make([][][]byte, batches)
	values := make([][][]byte, batches)
	for i := 0; i < 10; i++ {
		if i == 0 || !reuseKeys {
			keys[i] = make([][]byte, itemsPerBatch)
			for j := 0; j < itemsPerBatch; j++ {
				k := make([]byte, keySize)
				rand.Read(k)
				keys[i][j] = k
			}
		} else {
			keys[i] = keys[0]
		}
		values[i] = make([][]byte, itemsPerBatch)
		for j := 0; j < itemsPerBatch; j++ {
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
	db, last, err := New(logger, baseDir, defaultInitialSize, defaultBufferSize, defaultHistoryLen)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Put
	b, err := db.NewBatch(10)
	require.NoError(err)
	require.Zero(b.Prepare())
	b.Put("hello", []byte("world"))
	batch, err := b.Write()
	require.NoError(err)
	require.NotEqual(ids.Empty, batch)

	// Get
	v, err := db.Get("hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)

	// Restart
	require.NoError(db.Close())
	db, last, err = New(logger, baseDir, defaultInitialSize, defaultBufferSize, defaultHistoryLen)
	require.NoError(err)
	require.Equal(batch, last)

	// Get
	v, err = db.Get("hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)

	// Corrupt file
	require.NoError(db.Close())
	f, err := os.OpenFile(filepath.Join(baseDir, "0"), os.O_RDWR, os.ModeAppend)
	require.NoError(err)
	_, err = f.WriteString("corrupt")
	require.NoError(err)
	require.NoError(f.Close())

	// Restart
	db, last, err = New(logger, baseDir, defaultInitialSize, defaultBufferSize, defaultHistoryLen)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Get
	v, err = db.Get("hello")
	require.ErrorIs(err, database.ErrNotFound)
	require.NoError(db.Close())
}

func TestAppendDBPrune(t *testing.T) {
	// Prepare
	require := require.New(t)
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
	db, last, err := New(logger, baseDir, defaultInitialSize, defaultBufferSize, 10)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Insert 100 batches
	var lastBatch ids.ID
	for i := 0; i < 100; i++ {
		b, err := db.NewBatch(10)
		require.NoError(err)
		b.Prepare()
		switch {
		case i == 0:
			// Never modify again
			b.Put("hello", []byte("world"))
		case i < 99:
			for j := 0; j < 10; j++ {
				b.Put(strconv.Itoa(j), []byte(strconv.Itoa(i)))
			}
		default:
			b.Delete(strconv.Itoa(0))
		}
		lastBatch, err = b.Write()
		require.NoError(err)
		require.NotEqual(ids.Empty, lastBatch)
	}

	// Ensure data is correct
	v, err := db.Get("hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)
	for i := 0; i < 10; i++ {
		v, err = db.Get(strconv.Itoa(i))
		if i == 0 {
			require.ErrorIs(err, database.ErrNotFound)
		} else {
			require.NoError(err)
			require.Equal([]byte("98"), v)
		}
	}

	// Ensure files were pruned
	require.NoError(db.Close())
	files, err := os.ReadDir(baseDir)
	require.NoError(err)
	require.Len(files, 10) // 10 for the data files

	// Restart
	db, last, err = New(logger, baseDir, defaultInitialSize, defaultBufferSize, 10)
	require.NoError(err)
	require.Equal(lastBatch, last)

	// Ensure data is correct after restart
	v, err = db.Get("hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)
	for i := 0; i < 10; i++ {
		v, err = db.Get(strconv.Itoa(i))
		if i == 0 {
			require.ErrorIs(err, database.ErrNotFound)
		} else {
			require.NoError(err)
			require.Equal([]byte("98"), v)
		}
	}

	// Write to new batches
	for i := 0; i < 10; i++ {
		b, err := db.NewBatch(10)
		require.NoError(err)
		b.Prepare()
		for j := 0; j < 10; j++ {
			if i == 5 {
				b.Delete(strconv.Itoa(j))
			} else {
				b.Put(strconv.Itoa(j), []byte(strconv.Itoa(i)))
			}
		}
		lastBatch, err = b.Write()
		require.NoError(err)
		require.NotEqual(ids.Empty, lastBatch)
	}
	require.NoError(db.Close())
	files, err = os.ReadDir(baseDir)
	require.NoError(err)
	require.Len(files, 10) // 10 for the data files

	// Read from new batches
	db, last, err = New(logger, baseDir, defaultInitialSize, defaultBufferSize, 10)
	require.NoError(err)
	require.Equal(lastBatch, last)
	for i := 0; i < 10; i++ {
		v, err = db.Get(strconv.Itoa(i))
		require.NoError(err)
		require.Equal([]byte(strconv.Itoa(9)), v)
	}
	v, err = db.Get("hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)
	require.NoError(db.Close())
}

func TestAppendDBLarge(t *testing.T) {
	// Prepare
	require := require.New(t)
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
	db, last, err := New(logger, baseDir, defaultInitialSize, defaultBufferSize, 5)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Write 1M unique keys in 10 batches
	batches := 10
	batchSize := 1_000_000
	keys, values := randomKeyValues(batches, batchSize, 32, 32, false)
	checksums := make([]ids.ID, batches)
	for i := 0; i < batches; i++ {
		b, err := db.NewBatch(batchSize + 100)
		require.NoError(err)
		recycled := b.Prepare()
		if i < 5 {
			require.Zero(recycled)
		} else {
			require.Equal(batchSize, recycled)
		}
		for j := 0; j < batchSize; j++ {
			b.Put(string(keys[i][j]), values[i][j])
		}
		checksum, err := b.Write()
		require.NoError(err)

		// Ensure data is correct
		for j := 0; j < batchSize; j++ {
			v, err := db.Get(string(keys[i][j]))
			require.NoError(err)
			require.Equal(values[i][j], v)
		}
		checksums[i] = checksum
	}

	// Restart
	require.NoError(db.Close())
	db, last, err = New(logger, baseDir, defaultInitialSize, defaultBufferSize, 5)
	require.NoError(err)
	require.Equal(checksums[batches-1], last)

	// Ensure data is correct after restart
	for i := 0; i < batchSize; i++ {
		v, err := db.Get(string(keys[9][i]))
		require.NoError(err)
		require.Equal(values[9][i], v)
	}
	require.NoError(db.Close())

	// Create another database and ensure checksums match
	db2, last, err := New(logger, t.TempDir(), defaultInitialSize, defaultBufferSize, 5)
	require.NoError(err)
	require.Equal(ids.Empty, last)
	for i := 0; i < batches; i++ {
		b, err := db2.NewBatch(batchSize + 100)
		require.NoError(err)
		b.Prepare()
		for j := 0; j < batchSize; j++ {
			b.Put(string(keys[i][j]), values[i][j])
		}
		checksum, err := b.Write()
		require.NoError(err)
		require.Equal(checksums[i], checksum)
	}
}

func BenchmarkAppendDB(b *testing.B) {
	// Prepare
	require := require.New(b)
	logger := logging.NewLogger(
		"appenddb",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stdout,
			logging.Colors.ConsoleEncoder(),
		),
	)

	for _, reuse := range []bool{false, true} {
		batches := 10
		batchSize := 1_000_000
		keys, values := randomKeyValues(batches, batchSize, 32, 32, reuse)
		for _, bufferSize := range []int{4 * units.KiB, 64 * units.KiB, 128 * units.KiB, 256 * units.KiB, 1 * units.MiB} {
			b.Run(fmt.Sprintf("buffer=%d reuse=%t", bufferSize, reuse), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					// Create
					baseDir := b.TempDir()
					db, last, err := New(logger, baseDir, defaultInitialSize, defaultBufferSize, 5)
					require.NoError(err)
					require.Equal(ids.Empty, last)

					// Write 1M unique keys in 10 batches
					for j := 0; j < batches; j++ {
						b, err := db.NewBatch(batchSize + 100)
						require.NoError(err)
						start := time.Now()
						recycled := b.Prepare()
						prepareDuration := time.Since(start)
						start = time.Now()
						for k := 0; k < batchSize; k++ {
							b.Put(string(keys[j][k]), values[j][k])
						}
						putDuration := time.Since(start)
						start = time.Now()
						_, err = b.Write()
						logger.Info(
							"latency",
							zap.Duration("prepare", prepareDuration),
							zap.Int("recycled", recycled),
							zap.Duration("puts", putDuration),
							zap.Duration("write", time.Since(start)),
						)
						require.NoError(err)
					}

					// Cleanup
					require.NoError(db.Close())
					os.Remove(baseDir)
				}
			})
		}
	}
}

func BenchmarkPebbleDB(b *testing.B) {
	// Prepare
	require := require.New(b)

	for _, reuse := range []bool{false, true} {
		batches := 10
		batchSize := 1_000_000
		keys, values := randomKeyValues(batches, batchSize, 32, 32, reuse)
		b.Run(fmt.Sprintf("reuse=%t", reuse), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// Create
				baseDir := b.TempDir()
				db, _, err := pebble.New(baseDir, pebble.NewDefaultConfig())
				require.NoError(err)

				// Write 1M unique keys in 10 batches
				for j := 0; j < batches; j++ {
					b := db.NewBatch()
					for k := 0; k < batchSize; k++ {
						b.Put(keys[j][k], values[j][k])
					}
					require.NoError(b.Write())
				}

				// Cleanup
				require.NoError(db.Close())
				os.Remove(baseDir)
			}
		})
	}
}

// Takes ~200ms on Mac
func BenchmarkSortIDs(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		items := make([]string, 1_000_000)
		for i := 0; i < 1_000_000; i++ {
			id := ids.GenerateTestID()
			items[i] = string(id[:])
		}
		b.StartTimer()
		slices.Sort(items)
	}
}

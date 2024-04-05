package appenddb

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
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
	b.Prepare()
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

// TODO: add random reads and writes to test to ensure locations are updated right

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

	// Generate keys
	keys := make([]string, 10_000_000)
	values := make([][]byte, 10_000_000)
	for i := 0; i < 10_000_000; i++ {
		k := make([]byte, 32)
		rand.Read(k)
		keys[i] = string(k)
		v := make([]byte, 32)
		rand.Read(v)
		values[i] = v
	}

	b.ResetTimer()
	for _, bufferSize := range []int{4 * units.KiB, 64 * units.KiB, 128 * units.KiB, 256 * units.KiB, 1 * units.MiB} {
		b.Run(fmt.Sprintf("buffer=%d", bufferSize), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// Create
				baseDir := b.TempDir()
				db, last, err := New(logger, baseDir, defaultInitialSize, defaultBufferSize, 5)
				require.NoError(err)
				require.Equal(ids.Empty, last)

				// Write 1M unique keys in 10 batches
				lastIndex := 0
				for j := 0; j < 10; j++ {
					b, err := db.NewBatch(1_500_000)
					require.NoError(err)
					start := time.Now()
					b.Prepare()
					prepareDuration := time.Since(start)
					start = time.Now()
					for lastIndex < (j+1)*1_000_000 {
						b.Put(keys[lastIndex], values[lastIndex])
						lastIndex++
					}
					putDuration := time.Since(start)
					start = time.Now()
					_, err = b.Write()
					logger.Info(
						"latency",
						zap.Duration("prepare", prepareDuration),
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

func BenchmarkPebbleDB(b *testing.B) {
	// Prepare
	require := require.New(b)

	// Generate keys
	keys := make([][]byte, 10_000_000)
	values := make([][]byte, 10_000_000)
	for i := 0; i < 10_000_000; i++ {
		k := make([]byte, 32)
		rand.Read(k)
		keys[i] = k
		v := make([]byte, 32)
		rand.Read(v)
		values[i] = v
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create
		baseDir := b.TempDir()
		db, _, err := pebble.New(baseDir, pebble.NewDefaultConfig())
		require.NoError(err)

		// Write 1M unique keys in 10 batches
		lastIndex := 0
		for j := 0; j < 10; j++ {
			b := db.NewBatch()
			for lastIndex < (j+1)*1_000_000 {
				b.Put(keys[lastIndex], values[lastIndex])
				lastIndex++
			}
			require.NoError(b.Write())
		}

		// Cleanup
		require.NoError(db.Close())
		os.Remove(baseDir)
	}
}

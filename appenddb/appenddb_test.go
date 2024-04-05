package appenddb

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
	db, last, err := New(logger, baseDir, 100)
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
	db, last, err = New(logger, baseDir, 100)
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
	db, last, err = New(logger, baseDir, 100)
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
	db, last, err := New(logger, baseDir, 10)
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
	db, last, err = New(logger, baseDir, 10)
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
	db, last, err = New(logger, baseDir, 10)
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

	for i := 0; i < b.N; i++ {
		// Create
		baseDir := b.TempDir()
		db, last, err := New(logger, baseDir, 100)
		require.NoError(err)
		require.Equal(ids.Empty, last)

		// Write 1M unique keys
		b, err := db.NewBatch(1_000_000)
		require.NoError(err)
		b.Prepare()
		for k := 0; k < 1_000_000; k++ {
			b.Put(strconv.Itoa(k), []byte(strconv.Itoa(k)))
		}
		_, err = b.Write()
		require.NoError(err)

		// Cleanup
		require.NoError(db.Close())
		os.Remove(baseDir)
	}
}

func BenchmarkPebbleDB(b *testing.B) {
	// Prepare
	require := require.New(b)

	for i := 0; i < b.N; i++ {
		// Create
		baseDir := b.TempDir()
		db, _, err := pebble.New(baseDir, pebble.NewDefaultConfig())
		require.NoError(err)

		// Write 1M unique keys
		b := db.NewBatch()
		for k := 0; k < 1_000_000; k++ {
			b.Put([]byte(strconv.Itoa(k)), []byte(strconv.Itoa(k)))
		}
		require.NoError(b.Write())

		// Cleanup
		require.NoError(db.Close())
		os.Remove(baseDir)
	}
}

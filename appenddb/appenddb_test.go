package appenddb

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
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
	openBytes, movedFile := b.Prepare()
	require.Equal(int64(0), openBytes)
	require.False(movedFile)
	require.NoError(b.Put(ctx, "world", []byte("hello")))
	_, err = b.Write()
	require.NoError(err)

	// Modify recycled key
	b, err = db.NewBatch()
	require.NoError(err)
	openBytes, movedFile = b.Prepare()
	require.Equal(int64(0), openBytes) // no changes since last opened
	require.True(movedFile)
	require.NoError(b.Put(ctx, "hello", []byte("world2")))
	_, err = b.Write()
	require.NoError(err)

	// Delete recycled key
	b, err = db.NewBatch()
	require.NoError(err)
	openBytes, movedFile = b.Prepare()
	require.Equal(int64(0), openBytes)
	require.True(movedFile)
	require.NoError(b.Delete(ctx, "world"))
	checksum, err := b.Write()
	require.NoError(err)
	keys, alive, useless := db.Usage()

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

// func TestAppendDBPrune(t *testing.T) {
// 	// Prepare
// 	require := require.New(t)
// 	ctx := context.TODO()
// 	baseDir := t.TempDir()
// 	logger := logging.NewLogger(
// 		"appenddb",
// 		logging.NewWrappedCore(
// 			logging.Debug,
// 			os.Stdout,
// 			logging.Colors.ConsoleEncoder(),
// 		),
// 	)
// 	logger.Info("created directory", zap.String("path", baseDir))
//
// 	// Create
// 	db, last, err := New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 10)
// 	require.NoError(err)
// 	require.Equal(ids.Empty, last)
//
// 	// Insert 100 batches
// 	var lastBatch ids.ID
// 	for i := 0; i < 100; i++ {
// 		b, err := db.NewBatch()
// 		require.NoError(err)
// 		b.Prepare()
// 		switch {
// 		case i == 0:
// 			// Never modify again
// 			require.NoError(b.Put(ctx, "hello", []byte("world")))
// 		case i < 99:
// 			for j := 0; j < 10; j++ {
// 				require.NoError(b.Put(ctx, strconv.Itoa(j), []byte(strconv.Itoa(i))))
// 			}
// 		default:
// 			require.NoError(b.Delete(ctx, strconv.Itoa(0)))
// 		}
// 		lastBatch, err = b.Write()
// 		require.NoError(err)
// 		require.NotEqual(ids.Empty, lastBatch)
// 	}
//
// 	// Ensure data is correct
// 	v, err := db.Get(ctx, "hello")
// 	require.NoError(err)
// 	require.Equal([]byte("world"), v)
// 	for i := 0; i < 10; i++ {
// 		v, err = db.Get(ctx, strconv.Itoa(i))
// 		if i == 0 {
// 			require.ErrorIs(err, database.ErrNotFound)
// 		} else {
// 			require.NoError(err)
// 			require.Equal([]byte("98"), v)
// 		}
// 	}
//
// 	// Ensure files were pruned
// 	require.NoError(db.Close())
// 	files, err := os.ReadDir(baseDir)
// 	require.NoError(err)
// 	require.Len(files, 11) // 10 historical batches
//
// 	// Restart
// 	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 10)
// 	require.NoError(err)
// 	require.Equal(lastBatch, last)
//
// 	// Ensure data is correct after restart
// 	v, err = db.Get(ctx, "hello")
// 	require.NoError(err)
// 	require.Equal([]byte("world"), v)
// 	for i := 0; i < 10; i++ {
// 		v, err = db.Get(ctx, strconv.Itoa(i))
// 		if i == 0 {
// 			require.ErrorIs(err, database.ErrNotFound)
// 		} else {
// 			require.NoError(err)
// 			require.Equal([]byte("98"), v)
// 		}
// 	}
//
// 	// Write to new batches
// 	for i := 0; i < 10; i++ {
// 		b, err := db.NewBatch()
// 		require.NoError(err)
// 		b.Prepare()
// 		for j := 0; j < 10; j++ {
// 			if i == 5 {
// 				require.NoError(b.Delete(ctx, strconv.Itoa(j)))
// 			} else {
// 				require.NoError(b.Put(ctx, strconv.Itoa(j), []byte(strconv.Itoa(i))))
// 			}
// 		}
// 		lastBatch, err = b.Write()
// 		require.NoError(err)
// 		require.NotEqual(ids.Empty, lastBatch)
// 	}
// 	require.NoError(db.Close())
// 	files, err = os.ReadDir(baseDir)
// 	require.NoError(err)
// 	require.Len(files, 11) // 10 historical batches
//
// 	// Read from new batches
// 	db, last, err = New(logger, baseDir, defaultInitialSize, 10, defaultBufferSize, 10)
// 	require.NoError(err)
// 	require.Equal(lastBatch, last)
// 	for i := 0; i < 10; i++ {
// 		v, err = db.Get(ctx, strconv.Itoa(i))
// 		require.NoError(err)
// 		require.Equal([]byte(strconv.Itoa(9)), v)
// 	}
// 	v, err = db.Get(ctx, "hello")
// 	require.NoError(err)
// 	require.Equal([]byte("world"), v)
// 	require.NoError(db.Close())
// }
//
// func TestAppendDBLarge(t *testing.T) {
// 	for _, valueSize := range []int{32, minDiskValueSize * 2} { // ensure mem and mmap work
// 		t.Run(fmt.Sprintf("valueSize=%d", valueSize), func(t *testing.T) {
// 			// Prepare
// 			require := require.New(t)
// 			ctx := context.TODO()
// 			baseDir := t.TempDir()
// 			logger := logging.NewLogger(
// 				"appenddb",
// 				logging.NewWrappedCore(
// 					logging.Debug,
// 					os.Stdout,
// 					logging.Colors.ConsoleEncoder(),
// 				),
// 			)
// 			logger.Info("created directory", zap.String("path", baseDir))
//
// 			// Create
// 			batchSize := 10_000
// 			db, last, err := New(logger, baseDir, defaultInitialSize, batchSize, defaultBufferSize, 5)
// 			require.NoError(err)
// 			require.Equal(ids.Empty, last)
//
// 			// Write 1M unique keys in 10 batches
// 			batches := 10
// 			keys, values := randomKeyValues(batches, batchSize, 32, 32, 0)
// 			checksums := make([]ids.ID, batches)
// 			for i := 0; i < batches; i++ {
// 				b, err := db.NewBatch()
// 				require.NoError(err)
// 				recycled := b.Prepare()
// 				if i <= 5 {
// 					require.Zero(recycled)
// 				} else {
// 					require.Equal(batchSize, recycled)
// 				}
// 				for j := 0; j < batchSize; j++ {
// 					require.NoError(b.Put(ctx, string(keys[i][j]), values[i][j]))
// 				}
// 				checksum, err := b.Write()
// 				require.NoError(err)
//
// 				// Ensure data is correct
// 				for j := 0; j < batchSize; j++ {
// 					v, err := db.Get(ctx, string(keys[i][j]))
// 					require.NoError(err)
// 					require.Equal(values[i][j], v)
// 				}
// 				checksums[i] = checksum
// 			}
//
// 			// Restart
// 			require.NoError(db.Close())
// 			db, last, err = New(logger, baseDir, defaultInitialSize, batchSize, defaultBufferSize, 5)
// 			require.NoError(err)
// 			require.Equal(checksums[batches-1], last)
//
// 			// Ensure data is correct after restart
// 			for i := 0; i < batchSize; i++ {
// 				v, err := db.Get(ctx, string(keys[9][i]))
// 				require.NoError(err)
// 				require.Equal(values[9][i], v)
// 			}
// 			require.NoError(db.Close())
//
// 			// Create another database and ensure checksums match
// 			db2, last, err := New(logger, t.TempDir(), defaultInitialSize, batchSize, defaultBufferSize, 5)
// 			require.NoError(err)
// 			require.Equal(ids.Empty, last)
// 			for i := 0; i < batches; i++ {
// 				b, err := db2.NewBatch()
// 				require.NoError(err)
// 				b.Prepare()
// 				for j := 0; j < batchSize; j++ {
// 					require.NoError(b.Put(ctx, string(keys[i][j]), values[i][j]))
// 				}
// 				checksum, err := b.Write()
// 				require.NoError(err)
// 				require.Equal(checksums[i], checksum)
// 			}
// 		})
// 	}
// }
//
// func BenchmarkAppendDB(b *testing.B) {
// 	// Prepare
// 	require := require.New(b)
// 	ctx := context.TODO()
//
// 	batches := 10
// 	for _, batchSize := range []int{25_000, 50_000, 100_000, 500_000, 1_000_000} {
// 		for _, reuse := range []int{0, batchSize / 4, batchSize / 3, batchSize / 2, batchSize} {
// 			for _, historyLen := range []int{1, 5, 10} {
// 				keys, values := randomKeyValues(batches, batchSize, 32, 32, reuse)
// 				b.Run(fmt.Sprintf("keys=%d reuse=%d history=%d", batchSize, reuse, historyLen), func(b *testing.B) {
// 					for i := 0; i < b.N; i++ {
// 						db, last, err := New(logging.NoLog{}, b.TempDir(), defaultInitialSize, batchSize, defaultBufferSize, historyLen)
// 						require.NoError(err)
// 						require.Equal(ids.Empty, last)
// 						for j := 0; j < batches; j++ {
// 							b, err := db.NewBatch()
// 							require.NoError(err)
// 							b.Prepare()
// 							for k := 0; k < batchSize; k++ {
// 								require.NoError(b.Put(ctx, string(keys[j][k]), values[j][k]))
// 							}
// 							_, err = b.Write()
// 							require.NoError(err)
// 						}
// 						require.NoError(db.Close())
// 					}
// 				})
// 			}
// 		}
// 	}
// }
//
// func BenchmarkPebbleDB(b *testing.B) {
// 	// Prepare
// 	require := require.New(b)
//
// 	batches := 10
// 	for _, batchSize := range []int{25_000, 50_000, 100_000, 500_000, 1_000_000} {
// 		for _, reuse := range []int{0, batchSize / 4, batchSize / 3, batchSize / 2, batchSize} {
// 			keys, values := randomKeyValues(batches, batchSize, 32, 32, reuse)
// 			b.Run(fmt.Sprintf("keys=%d reuse=%d", batchSize, reuse), func(b *testing.B) {
// 				for i := 0; i < b.N; i++ {
// 					db, _, err := pebble.New(b.TempDir(), pebble.NewDefaultConfig())
// 					require.NoError(err)
// 					for j := 0; j < batches; j++ {
// 						b := db.NewBatch()
// 						for k := 0; k < batchSize; k++ {
// 							b.Put(keys[j][k], values[j][k])
// 						}
// 						require.NoError(b.Write())
// 					}
// 					require.NoError(db.Close())
// 				}
// 			})
// 		}
// 	}
// }

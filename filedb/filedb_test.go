package filedb

import (
	"context"
	"crypto/rand"
	"fmt"
	"runtime"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestFileDB(t *testing.T) {
	require := require.New(t)
	db := New(t.TempDir(), true, 1024, 2*units.MiB)

	v, err := db.Get("test", "1")
	require.ErrorIs(err, database.ErrNotFound)
	require.Empty(v)

	require.NoError(db.Put("test", "1", []byte("2")))

	v, err = db.Get("test", "1")
	require.NoError(err)
	require.Equal([]byte("2"), v)

	require.NoError(db.Put("test", "2", []byte("3")))

	v, err = db.Get("test", "2")
	require.Nil(err)
	require.Equal([]byte("3"), v)

	require.NoError(db.Remove("test"))

	// Cache still works
	v, err = db.Get("test", "1")
	require.NoError(err)
	require.Equal([]byte("2"), v)
	v, err = db.Get("test", "2")
	require.Nil(err)
	require.Equal([]byte("3"), v)

	// Clear cache
	db.fileCache.Flush()
	v, err = db.Get("test", "1")
	require.ErrorIs(err, database.ErrNotFound)
	require.Empty(v)
	v, err = db.Get("test", "2")
	require.ErrorIs(err, database.ErrNotFound)
	require.Empty(v)
}

func BenchmarkFileDB(b *testing.B) {
	for _, sync := range []bool{true, false} {
		b.Run(fmt.Sprintf("sync=%v", sync), func(b *testing.B) {
			b.StopTimer()
			db := New(b.TempDir(), sync, 1024, 32*units.MiB)
			msg := make([]byte, 1.5*units.MiB)
			_, err := rand.Read(msg)
			if err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			for i := 0; i < b.N; i++ {
				if err := db.Put("test", fmt.Sprintf("%d", i), msg); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkFileDBConcurrent(b *testing.B) {
	for _, sync := range []bool{true, false} {
		b.Run(fmt.Sprintf("sync=%v", sync), func(b *testing.B) {
			b.StopTimer()
			db := New(b.TempDir(), sync, 1024, 32*units.MiB)
			msg := make([]byte, 1.5*units.MiB)
			_, err := rand.Read(msg)
			if err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			g, _ := errgroup.WithContext(context.TODO())
			g.SetLimit(runtime.NumCPU())
			for i := 0; i < b.N; i++ {
				ti := i
				g.Go(func() error {
					return db.Put("test", fmt.Sprintf("%d", ti), msg)
				})
			}
			if err := g.Wait(); err != nil {
				b.Fatal(err)
			}
		})
	}
}

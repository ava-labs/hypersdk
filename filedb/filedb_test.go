package filedb

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestFileDB(t *testing.T) {
	require := require.New(t)
	db := New(t.TempDir(), true, 1024, 2*units.MiB)

	v, err := db.Get("1")
	require.ErrorIs(err, database.ErrNotFound)
	require.Empty(v)

	require.NoError(db.Put("1", []byte("2")))

	v, err = db.Get("1")
	require.NoError(err)
	require.Equal([]byte("2"), v)

	require.NoError(db.Put("2", []byte("3")))

	v, err = db.Get("2")
	require.Nil(err)
	require.Equal([]byte("3"), v)

	require.NoError(db.Remove("1"))
	require.NoError(db.Remove("2"))

	v, err = db.Get("1")
	require.ErrorIs(err, database.ErrNotFound)
	require.Empty(v)
	v, err = db.Get("2")
	require.ErrorIs(err, database.ErrNotFound)
	require.Empty(v)

	require.Zero(db.lm.Locks())
}

func TestFileDBCorruption(t *testing.T) {
	require := require.New(t)
	dbDir := t.TempDir()
	db := New(dbDir, true, 1024, 2*units.MiB)

	v, err := db.Get("1")
	require.ErrorIs(err, database.ErrNotFound)
	require.Empty(v)

	require.NoError(db.Put("1", []byte("2")))

	v, err = db.Get("1")
	require.NoError(err)
	require.Equal([]byte("2"), v)

	// Corrupt file with invalid length
	f, err := os.Create(filepath.Join(dbDir, "1"))
	require.NoError(err)
	_, err = f.Write([]byte{})
	require.NoError(err)
	require.NoError(f.Close())

	db.fileCache.Flush()
	v, err = db.Get("1")
	require.ErrorIs(err, ErrCorrupt)

	// Corrupt file with invalid data
	f, err = os.Create(filepath.Join(dbDir, "1"))
	require.NoError(err)
	msg := make([]byte, 1.5*units.MiB)
	_, err = rand.Read(msg)
	require.NoError(err)
	_, err = f.Write(msg)
	require.NoError(err)
	require.NoError(f.Close())

	v, err = db.Get("1")
	require.ErrorIs(err, ErrCorrupt)
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
				if err := db.Put(fmt.Sprintf("%d", i), msg); err != nil {
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
					return db.Put(fmt.Sprintf("%d", ti), msg)
				})
			}
			if err := g.Wait(); err != nil {
				b.Fatal(err)
			}
		})
	}
}

func BenchmarkPebbleDB(b *testing.B) {
	b.StopTimer()
	db, _, err := pebble.New(b.TempDir(), pebble.NewDefaultConfig())
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	msg := make([]byte, 1.5*units.MiB)
	if _, err := rand.Read(msg); err != nil {
		b.Fatal(err)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := db.Put([]byte(fmt.Sprintf("%d", i)), msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPebbleDBConcurrent(b *testing.B) {
	b.StopTimer()
	db, _, err := pebble.New(b.TempDir(), pebble.NewDefaultConfig())
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	msg := make([]byte, 1.5*units.MiB)
	if _, err := rand.Read(msg); err != nil {
		b.Fatal(err)
	}
	b.StartTimer()
	g, _ := errgroup.WithContext(context.TODO())
	g.SetLimit(runtime.NumCPU())
	for i := 0; i < b.N; i++ {
		ti := i
		g.Go(func() error {
			return db.Put([]byte(fmt.Sprintf("%d", ti)), msg)
		})
	}
	if err := g.Wait(); err != nil {
		b.Fatal(err)
	}
}

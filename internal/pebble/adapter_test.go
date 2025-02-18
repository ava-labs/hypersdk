// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pebble

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// TestDeleteRangeImplementations compares the behavior of our fallback implementation
// with the native Pebble DeleteRange implementation.
func TestDeleteRangeImplementations(t *testing.T) {
	r := require.New(t)
	pebbleDB, err := New(t.TempDir(), NewDefaultConfig(), prometheus.NewRegistry())
	r.NoError(err)
	defer pebbleDB.Close()

	// memdb with fallback implementation
	memDB := WithExtendedDatabase(memdb.New())

	testCases := []struct {
		name     string
		setup    func(db database.Database) error
		start    []byte
		end      []byte
		validate func(db database.Database)
	}{
		{
			name: "empty range",
			setup: func(_ database.Database) error {
				return nil
			},
			start: []byte("test_a"),
			end:   []byte("test_b"),
			validate: func(db database.Database) {
				// Verify no keys exist in the range
				iter := db.NewIteratorWithStartAndPrefix([]byte("a"), []byte("test_"))
				defer iter.Release()
				r.False(iter.Next())
			},
		},
		{
			name: "single key in range",
			setup: func(db database.Database) error {
				return db.Put([]byte("key1"), []byte("value1"))
			},
			start: []byte("key1"),
			end:   []byte("key2"),
			validate: func(db database.Database) {
				// Verify the key was deleted
				has, err := db.Has([]byte("key1"))
				r.NoError(err)
				r.False(has)
			},
		},
		{
			name: "multiple consecutive keys",
			setup: func(db database.Database) error {
				for i := 0; i < 100; i++ {
					key := []byte(fmt.Sprintf("key%03d", i))
					if err := db.Put(key, []byte("value")); err != nil {
						return err
					}
				}
				return nil
			},
			start: []byte("key020"),
			end:   []byte("key080"),
			validate: func(db database.Database) {
				// Start key should be deleted
				hasStartKey, err := db.Has([]byte("key020"))
				r.NoError(err)
				r.False(hasStartKey)

				// End key should not be deleted
				hasEndKey, err := db.Has([]byte("key080"))
				r.NoError(err)
				r.True(hasEndKey)

				// Verify keys in range 20 to 79 deleted
				for i := 20; i <= 79; i++ {
					key := []byte(fmt.Sprintf("key%03d", i))
					hasKey, err := db.Has(key)
					r.NoError(err)
					r.False(hasKey)
				}

				it := db.NewIteratorWithStart([]byte("key080"))
				defer it.Release()

				for it.Next() {
					key := it.Key()
					has, err := db.Has(key)
					r.NoError(err)
					r.True(has)
				}
			},
		},
		{
			name: "start key is not present",
			setup: func(db database.Database) error {
				keys := []string{"key010", "key050", "key060", "key090"}
				for _, key := range keys {
					if err := db.Put([]byte(key), []byte("value")); err != nil {
						return err
					}
				}
				return nil
			},
			start: []byte("key020"),
			end:   []byte("key090"),
			validate: func(db database.Database) {
				delKeys := [][]byte{[]byte("key050"), []byte("key060")}
				for _, key := range delKeys {
					has, err := db.Has(key)
					r.NoError(err)
					r.False(has)
				}

				endKey, err := db.Has([]byte("key090"))
				r.NoError(err)
				r.True(endKey)
			},
		},
		{
			name: "end key is not present",
			setup: func(db database.Database) error {
				keys := []string{"key010", "key050", "key060", "key090"}
				for _, key := range keys {
					if err := db.Put([]byte(key), []byte("value")); err != nil {
						return err
					}
				}
				return nil
			},
			start: []byte("key010"),
			end:   []byte("key099"),
			validate: func(db database.Database) {
				// all keys should be deleted
				delKeys := [][]byte{[]byte("key010"), []byte("key050"), []byte("key060"), []byte("key090")}
				for _, key := range delKeys {
					has, err := db.Has(key)
					r.NoError(err)
					r.False(has)
				}
			},
		},
	}

	// Run each test case against both implementations
	for _, tc := range testCases {
		t.Run(tc.name+"_Pebble", func(_ *testing.T) {
			r.NoError(tc.setup(pebbleDB))
			r.NoError(pebbleDB.DeleteRange(tc.start, tc.end))
			tc.validate(pebbleDB)
		})

		t.Run(tc.name+"_Fallback", func(_ *testing.T) {
			r.NoError(tc.setup(memDB))
			r.NoError(memDB.DeleteRange(tc.start, tc.end))
			tc.validate(memDB)
		})
	}
}

func setupKeyPatterns(pattern string) []string {
	n := 1000
	switch pattern {
	case "sequential":
		// Create a dense sequence of keys with no gaps
		keys := make([]string, n)
		for i := 0; i < n; i++ {
			keys[i] = fmt.Sprintf("key%08d", i)
		}
		return keys
	case "sparse":
		// Create keys with random gaps between them
		keys := make([]string, n)
		for i := 0; i < n; i++ {
			gap := rand.Intn(10) + 1 //nolint:gosec
			keys[i] = fmt.Sprintf("key%08d", i*gap)
		}
		return keys
	default:
		panic("unknown pattern: " + pattern)
	}
}

// BenchmarkPebbleDeleteRange_Sequential tests Pebble's native DeleteRange
func BenchmarkPebbleDeleteRange_Sequential(b *testing.B) {
	r := require.New(b)
	dir := b.TempDir()
	pebbleDB, err := New(dir, NewDefaultConfig(), prometheus.NewRegistry())
	r.NoError(err)
	defer pebbleDB.Close()

	keys := setupKeyPatterns("sequential")
	startKey := []byte(keys[len(keys)/4]) // Start at 25% mark
	endKey := []byte(keys[len(keys)*3/4]) // End at 75% mark

	b.StopTimer()
	for _, key := range keys {
		if err := pebbleDB.Put([]byte(key), []byte("value")); err != nil {
			r.NoError(err)
		}
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := pebbleDB.DeleteRange(startKey, endKey); err != nil {
			r.NoError(err)
		}
	}
}

// BenchmarkPebbleDeleteRange_Sparse tests Pebble's native DeleteRange
func BenchmarkPebbleDeleteRange_Sparse(b *testing.B) {
	r := require.New(b)
	dir := b.TempDir()
	pebbleDB, err := New(dir, NewDefaultConfig(), prometheus.NewRegistry())
	r.NoError(err)
	defer pebbleDB.Close()

	// Setup sparse keys
	keys := setupKeyPatterns("sparse")
	startKey := []byte(keys[len(keys)/4])
	endKey := []byte(keys[len(keys)*3/4])

	b.StopTimer()
	for _, key := range keys {
		if err := pebbleDB.Put([]byte(key), []byte("value")); err != nil {
			r.NoError(err)
		}
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := pebbleDB.DeleteRange(startKey, endKey); err != nil {
			r.NoError(err)
		}
	}
}

// BenchmarkFallbackDeleteRange_Sequential tests our fallback DeleteRange
func BenchmarkFallbackDeleteRange_Sequential(b *testing.B) {
	r := require.New(b)
	memDB := WithExtendedDatabase(memdb.New())

	keys := setupKeyPatterns("sequential")
	startKey := []byte(keys[len(keys)/4])
	endKey := []byte(keys[len(keys)*3/4])

	b.StopTimer()
	for _, key := range keys {
		if err := memDB.Put([]byte(key), []byte("value")); err != nil {
			r.NoError(err)
		}
	}

	_, ok := memDB.(*fallbackRangeDB)
	r.True(ok)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := memDB.DeleteRange(startKey, endKey); err != nil {
			r.NoError(err)
		}
	}
}

// BenchmarkFallbackDeleteRange_Sparse tests our fallback DeleteRange
func BenchmarkFallbackDeleteRange_Sparse(b *testing.B) {
	r := require.New(b)
	memDB := WithExtendedDatabase(memdb.New())

	keys := setupKeyPatterns("sparse")
	startKey := []byte(keys[len(keys)/4])
	endKey := []byte(keys[len(keys)*3/4])

	b.StopTimer()
	for _, key := range keys {
		if err := memDB.Put([]byte(key), []byte("value")); err != nil {
			r.NoError(err)
		}
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := memDB.DeleteRange(startKey, endKey); err != nil {
			r.Error(err)
		}
	}
}

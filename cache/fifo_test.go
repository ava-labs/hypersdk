// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFIFOCache(t *testing.T) {
	limit := 10

	tests := []struct {
		name   string
		maxVal int
		check  func(t *testing.T, cache *FIFO[int, int])
	}{
		{
			name:   "inserting up to limit works",
			maxVal: limit - 1,
		},
		{
			name:   "inserting after limit cleans first",
			maxVal: limit,
			check: func(t *testing.T, cache *FIFO[int, int]) {
				require := require.New(t)
				_, ok := cache.Get(0)
				require.False(ok)
				v, ok := cache.Get(1)
				require.True(ok)
				require.Equal(1, v)
			},
		},
		{
			name:   "inserting before limit can insert again",
			maxVal: limit - 2,
			check: func(t *testing.T, cache *FIFO[int, int]) {
				require := require.New(t)
				exists := cache.Put(limit-1, limit-1)
				require.False(exists)
				exists = cache.Put(limit, limit)
				require.False(exists)
				_, ok := cache.Get(0)
				require.False(ok)
			},
		},
		{
			name:   "inserting existing value doesn't free value",
			maxVal: limit - 1,
			check: func(t *testing.T, cache *FIFO[int, int]) {
				require := require.New(t)
				v, ok := cache.Get(0)
				require.True(ok)
				require.Equal(0, v)
				exists := cache.Put(0, 10)
				require.True(exists)
				v, ok = cache.Get(0)
				require.True(ok)
				require.Equal(10, v)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cache, err := NewFIFO[int, int](limit)
			require.NoError(err)

			for i := 0; i <= tt.maxVal; i++ {
				exists := cache.Put(i, i)
				require.False(exists)
				v, ok := cache.Get(i)
				require.True(ok)
				require.Equal(i, v)
			}

			if tt.check != nil {
				tt.check(t, cache)
			}
		})
	}
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cache

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFIFOCache(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		maxVal   int
		expected error
		check    func(t *testing.T, cache *FIFO[int, int])
	}{
		{
			name:     "empty cache fails",
			limit:    0,
			expected: errors.New("maxSize must be greater than 0"),
		},
		{
			name:   "inserting up to limit works",
			limit:  10,
			maxVal: 9,
		},
		{
			name:   "inserting after limit cleans first",
			limit:  10,
			maxVal: 10,
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
			limit:  10,
			maxVal: 8,
			check: func(t *testing.T, cache *FIFO[int, int]) {
				require := require.New(t)
				exists := cache.Put(9, 9)
				require.False(exists)
				exists = cache.Put(10, 10)
				require.False(exists)
				_, ok := cache.Get(0)
				require.False(ok)
			},
		},
		{
			name:   "inserting existing value doesn't free value",
			limit:  10,
			maxVal: 9,
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

			cache, err := NewFIFO[int, int](tt.limit)
			require.Equal(tt.expected, err)
			if err != nil {
				return
			}

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

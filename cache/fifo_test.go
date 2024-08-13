// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cache

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFIFOCacheInsertion(t *testing.T) {
	type put struct {
		kv     int
		exists bool
	}

	type get struct {
		k  int
		ok bool
	}

	tests := []struct {
		name string
		ops  []interface{}
	}{
		{
			name: "inserting up to limit works",
			ops: []interface{}{
				put{
					kv:     0,
					exists: false,
				},
				put{
					kv:     1,
					exists: false,
				},
			},
		},
		{
			name: "inserting after limit cleans first",
			ops: []interface{}{
				put{
					kv:     0,
					exists: false,
				},
				put{
					kv:     1,
					exists: false,
				},
				put{
					kv:     2,
					exists: false,
				},
				get{
					k:  0,
					ok: false,
				},
			},
		},
		{
			name: "no element removed when cache is exactly full",
			ops: []interface{}{
				put{
					kv:     0,
					exists: false,
				},
				put{
					kv:     1,
					exists: false,
				},
				get{
					k:  0,
					ok: true,
				},
				get{
					k:  1,
					ok: true,
				},
			},
		},
		{
			name: "no element removed when the cache is less than full",
			ops: []interface{}{
				put{
					kv:     0,
					exists: false,
				},
				get{
					k:  0,
					ok: true,
				},
			},
		},
		{
			name: "inserting existing value when full doesn't free value",
			ops: []interface{}{
				put{
					kv:     0,
					exists: false,
				},
				put{
					kv:     1,
					exists: false,
				},
				put{
					kv:     0,
					exists: true,
				},
				get{
					k:  0,
					ok: true,
				},
			},
		},
		{
			name: "elements removed in FIFO order when cache overfills",
			ops: []interface{}{
				put{
					kv:     0,
					exists: false,
				},
				put{
					kv:     1,
					exists: false,
				},
				put{
					kv:     2,
					exists: false,
				},
				get{
					k:  0,
					ok: false,
				},
				put{
					kv:     3,
					exists: false,
				},
				get{
					k:  1,
					ok: false,
				},
			},
		},
		{
			name: "elements removed in FIFO order and not LRU",
			ops: []interface{}{
				put{
					kv:     0,
					exists: false,
				},
				put{
					kv:     1,
					exists: false,
				},
				put{
					kv:     0,
					exists: true,
				},
				get{
					k:  0,
					ok: true,
				},
				put{
					kv:     2,
					exists: false,
				},
				get{
					k:  0,
					ok: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cache, err := NewFIFO[int, int](2)
			require.NoError(err)

			for _, opIntf := range tt.ops {
				switch op := opIntf.(type) {
				case put:
					exists := cache.Put(op.kv, op.kv)
					require.Equal(op.exists, exists)
				case get:
					val, ok := cache.Get(op.k)
					require.Equal(op.ok, ok)
					if ok {
						require.Equal(op.k, val)
					}
				default:
					require.Fail("op can only be a put or a get")
				}
			}
		})
	}
}

func TestEmptyCacheFails(t *testing.T) {
	require := require.New(t)
	_, err := NewFIFO[int, int](0)
	expectedErr := errors.New("maxSize must be greater than 0")
	require.Equal(expectedErr, err)
}

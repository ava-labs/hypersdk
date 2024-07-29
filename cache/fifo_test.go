// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cache

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFIFOCacheInsertion(t *testing.T) {
	limit := 10

	type put struct {
		i      int
		exists bool
	}

	type get struct {
		i  int
		ok bool
	}

	tests := []struct {
		name string
		ops  []interface{}
	}{
		{
			name: "inserting 10 elements works",
			ops: func() []interface{} {
				ops := make([]interface{}, 20)
				for i := 0; i < limit; i++ {
					ops[2*i] = put{
						i:      i,
						exists: false,
					}
					ops[2*i+1] = get{
						i:  i,
						ok: true,
					}
				}
				return ops
			}(),
		},
		{
			name: "inserting after limit cleans first",
			ops: func() []interface{} {
				ops := make([]interface{}, 23)
				for i := 0; i < limit; i++ {
					ops[2*i] = put{
						i:      i,
						exists: false,
					}
					ops[2*i+1] = get{
						i:  i,
						ok: true,
					}
				}
				ops[2*limit] = put{
					i:      10,
					exists: false,
				}
				ops[2*limit+1] = get{
					i:  0,
					ok: false,
				}
				ops[2*limit+2] = get{
					i:  1,
					ok: true,
				}
				return ops
			}(),
		},
		{
			name: "no elements removed when cache is exactly full",
			ops: func() []interface{} {
				ops := make([]interface{}, 20)
				for i := 0; i < 9; i++ {
					ops[2*i] = put{
						i:      i,
						exists: false,
					}
					ops[2*i+1] = get{
						i:  i,
						ok: true,
					}
				}
				ops[2*9] = put{
					i:      9,
					exists: false,
				}
				ops[2*9+1] = get{
					i:  9,
					ok: true,
				}
				return ops
			}(),
		},
		{
			name: "no elements removed when the cache is less than full",
			ops: func() []interface{} {
				ops := make([]interface{}, 12)
				for i := 0; i < 5; i++ {
					ops[2*i] = put{
						i:      i,
						exists: false,
					}
					ops[2*i+1] = get{
						i:  i,
						ok: true,
					}
				}
				ops[2*5] = put{
					i:      5,
					exists: false,
				}
				ops[2*5+1] = get{
					i:  0,
					ok: true,
				}
				return ops
			}(),
		},
		{
			name: "inserting existing value when full doesn't free value",
			ops: func() []interface{} {
				ops := make([]interface{}, 20)
				for i := 0; i < limit; i++ {
					ops[2*i] = put{
						i:      i,
						exists: false,
					}
					ops[2*i+1] = get{
						i:  i,
						ok: true,
					}
				}
				ops[2*9] = put{
					i:      0,
					exists: true,
				}
				ops[2*9+1] = get{
					i:  0,
					ok: true,
				}
				return ops
			}(),
		},
		{
			name: "elements removed in FIFO order when cache overfills",
			ops: func() []interface{} {
				ops := make([]interface{}, 40)
				for i := 0; i < limit; i++ {
					ops[2*i] = put{
						i:      i,
						exists: false,
					}
					ops[2*i+1] = get{
						i:  i,
						ok: true,
					}
				}
				for i := 0; i < limit; i++ {
					ops[2*(i+limit)] = put{
						i:      (2 * (i + limit)),
						exists: false,
					}
					ops[2*(i+limit)+1] = get{
						ok: false,
					}
				}
				return ops
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			cache, err := NewFIFO[int, int](limit)
			require.NoError(err)

			for _, op := range tt.ops {
				if put, ok := op.(put); ok {
					exists := cache.Put(put.i, put.i)
					require.Equal(put.exists, exists)
				} else if get, ok := op.(get); ok {
					val, ok := cache.Get(get.i)
					require.Equal(get.ok, ok)
					require.Equal(get.i, val)
				} else {
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

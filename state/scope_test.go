// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state/dbtest"
)

var (
	testKey = []byte("key")
	testVal = []byte("value")

	key1    = keys.EncodeChunks([]byte("key1"), 1)
	key1str = string(key1)
	key2    = keys.EncodeChunks([]byte("key2"), 2)
	key2str = string(key2)
	key3    = keys.EncodeChunks([]byte("key3"), 3)
	key3str = string(key3)
)

func TestEmptyDefaultScope(t *testing.T) {
	r := require.New(t)
	ctx := context.TODO()
	db := dbtest.NewTestDB()

	scope := NewDefaultScope(
		Keys{},
		db,
	)

	v, err := scope.GetValue(ctx, string(testKey))
	r.ErrorIs(err, database.ErrNotFound)
	r.Nil(v)
}

func TestDefaultScopeValues(t *testing.T) {
	tests := []struct {
		key   string
		value []byte
		err   error
	}{
		{
			key:   string(testKey),
			value: testVal,
			err:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			r := require.New(t)
			ctx := context.TODO()
			db := dbtest.NewTestDB()

			r.NoError(db.Insert(ctx, []byte(tt.key), tt.value))

			scope := NewDefaultScope(
				Keys{tt.key: All},
				db,
			)

			v, err := scope.GetValue(ctx, tt.key)
			r.ErrorIs(err, tt.err)
			r.Equal(tt.value, v)
		})
	}
}

func TestTieredScopeInitialization(t *testing.T) {
	t.Run("values with no suffixes", func(t *testing.T) {
		r := require.New(t)
		ims := map[string][]byte{
			key1str: testVal,
			key2str: testVal,
			key3str: testVal,
		}
		_, err := NewTieredScope(
			Keys{},
			ims,
			nil,
			0,
			0,
		)
		r.ErrorIs(err, ErrValueTooShort)
	})

	t.Run("values have suffixes", func(t *testing.T) {
		r := require.New(t)
		ims := map[string][]byte{
			key1str: keys.EncodeSuffix(testVal, 0),
			key2str: keys.EncodeSuffix(testVal, 0),
			key3str: keys.EncodeSuffix(testVal, 0),
		}
		_, err := NewTieredScope(
			Keys{},
			ims,
			nil,
			0,
			0,
		)
		r.NoError(err)
	})
}

func TestEmptyTieredScope(t *testing.T) {
	ctx := context.TODO()
	db := dbtest.NewTestDB()

	tests := []struct {
		name string
		keys Keys
		err  error
	}{
		{
			name: "no state keys",
			err:  ErrOptimisticReadFailed,
		},
		{
			name: "insufficient state key permissions",
			keys: Keys{string(testKey): ReadFromMemory},
			err:  ErrOptimisticReadFailed,
		},
		{
			name: "sufficient state key permissions",
			keys: Keys{string(testKey): ReadFromStorage},
			err:  database.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			scope, err := NewTieredScope(
				tt.keys,
				map[string][]byte{},
				db,
				0,
				0,
			)
			r.NoError(err)

			v, err := scope.GetValue(ctx, string(testKey))
			r.ErrorIs(err, tt.err)
			r.Nil(v)
		})
	}
}

func TestTieredScopeLocalState(t *testing.T) {
	ctx := context.TODO()
	db := dbtest.NewTestDB()
	tests := []struct {
		key string
	}{
		{
			key: key1str,
		},
		{
			key: key2str,
		},
		{
			key: key3str,
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			r := require.New(t)
			scope, err := NewTieredScope(
				Keys{},
				map[string][]byte{
					tt.key: keys.EncodeSuffix(testVal, 0),
				},
				db,
				0,
				0,
			)
			r.NoError(err)

			v, err := scope.GetValue(ctx, tt.key)
			r.NoError(err)
			r.Equal(testVal, v)
		})
	}
}

// LocalMemory is not populated in this test
func TestTieredScopePersistentState(t *testing.T) {
	ctx := context.TODO()
	tests := []struct {
		// Scope construction values
		name string
		sk   Keys
		// This mapping is copied into a new instance of TestDB
		st          map[string][]byte
		blockHeight uint64
		epsilon     uint64

		// Scope fetching values
		key string
		val []byte
		err error
	}{
		{
			name: "value does not exist and ReadFromMemory",
			sk:   Keys{key1str: ReadFromMemory},
			st:   map[string][]byte{},
			key:  key1str,
			val:  nil,
			err:  ErrOptimisticReadFailed,
		},
		{
			name: "value does not exist and ReadFromStorage",
			sk:   Keys{key1str: ReadFromStorage},
			st:   map[string][]byte{},
			key:  key1str,
			val:  nil,
			err:  database.ErrNotFound,
		},
		{
			name: "value exists, but ReadFromMemory and is value is old",
			sk:   Keys{key1str: ReadFromMemory},
			st: map[string][]byte{
				key1str: keys.EncodeSuffix(testVal, 5),
			},
			blockHeight: 7,
			epsilon:     1,
			key:         key1str,
			val:         nil,
			err:         ErrOptimisticReadFailed,
		},
		{
			name: "value exists, but ReadFromMemory and is value is young",
			sk:   Keys{key1str: ReadFromMemory},
			st: map[string][]byte{
				key1str: keys.EncodeSuffix(testVal, 5),
			},
			blockHeight: 6,
			epsilon:     1,
			key:         key1str,
			val:         testVal,
			err:         nil,
		},
		{
			name: "correct permissions",
			sk:   Keys{key1str: ReadFromStorage},
			st: map[string][]byte{
				key1str: keys.EncodeSuffix(testVal, 0),
			},
			blockHeight: 0,
			epsilon:     1,
			key:         key1str,
			val:         testVal,
			err:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			db := dbtest.NewTestDB()

			for k, v := range tt.st {
				r.NoError(db.Insert(ctx, []byte(k), v))
			}

			scope, err := NewTieredScope(
				tt.sk,
				map[string][]byte{},
				db,
				tt.blockHeight,
				tt.epsilon,
			)
			r.NoError(err)

			v, err := scope.GetValue(ctx, tt.key)
			r.ErrorIs(err, tt.err)
			r.Equal(tt.val, v)
		})
	}
}

func TestTieredScopeMixedState(t *testing.T) {
	ctx := context.TODO()

	t.Run("local state updates on persistent state fetch", func(t *testing.T) {
		r := require.New(t)

		db := dbtest.NewTestDB()
		r.NoError(db.Insert(ctx, key1, keys.EncodeSuffix(testVal, 0)))

		scope, err := NewTieredScope(
			Keys{key1str: ReadFromStorage},
			map[string][]byte{},
			db,
			1,
			1,
		)
		r.NoError(err)

		v, err := scope.GetValue(ctx, key1str)
		r.NoError(err)
		r.Equal(testVal, v)

		localVal, ok := scope.localState[key1str]
		r.True(ok)
		r.Equal(testVal, localVal)
	})
}

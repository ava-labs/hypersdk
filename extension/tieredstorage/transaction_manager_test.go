// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tieredstorage

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/consts"
)

var (
	testKey = "testKey"
	// BLk height is 0
	testVal = binary.BigEndian.AppendUint64([]byte("testVal"), 0)
)

func TestTransactionManagerHotKeys(t *testing.T) {
	tests := []struct {
		name        string
		rawState    map[string][]byte
		epsilon     uint64
		blockHeight uint64
		key         string
		isHotKey    bool
	}{
		{
			name:     "empty state has no hot keys",
			rawState: map[string][]byte{},
		},
		{
			name:        "state with hot key",
			rawState:    map[string][]byte{testKey: testVal},
			epsilon:     1,
			blockHeight: 1,
			key:         testKey,
			isHotKey:    true,
		},
		{
			name:        "state with hot key and no underflow",
			rawState:    map[string][]byte{testKey: testVal},
			epsilon:     2,
			blockHeight: 1,
			key:         testKey,
			isHotKey:    true,
		},
		{
			name:        "state with cold key",
			rawState:    map[string][]byte{testKey: testVal},
			epsilon:     2,
			blockHeight: 3,
			key:         testKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			tm := newTieredStorageTransactionManager(Config{Epsilon: tt.epsilon})

			_, err := tm.ExecutableState(tt.rawState, tt.blockHeight)
			r.NoError(err)

			r.Equal(tt.isHotKey, tm.IsHotKey(tt.key))
		})
	}
}

func TestTransactionManagerShim(t *testing.T) {
	tests := []struct {
		name     string
		rawState map[string][]byte
		err      error
	}{
		{
			name: "rawDB with prefixes",
			rawState: map[string][]byte{
				testKey: testVal,
			},
		},
		{
			name: "rawDB with no prefixes",
			rawState: map[string][]byte{
				testKey: testVal[:len(testVal)-consts.Uint64Len],
			},
			err: errValueTooShortForSuffix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			tm := newTieredStorageTransactionManager(Config{Epsilon: 1})

			_, err := tm.ExecutableState(tt.rawState, 1)
			r.Equal(tt.err, err)
		})
	}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/consts"
)

var (
	key     = binary.BigEndian.AppendUint16([]byte("key"), 0)
	keyStr  = string(key)
	key1    = binary.BigEndian.AppendUint16([]byte("key1"), 1)
	key1Str = string(key1)
	key2    = binary.BigEndian.AppendUint16([]byte("key2"), 2)
	key2Str = string(key2)
	key3    = binary.BigEndian.AppendUint16([]byte("key3"), 3)
	key3Str = string(key3)

	value  = binary.BigEndian.AppendUint64([]byte("value"), 0)
	value1 = binary.BigEndian.AppendUint64([]byte("value1"), 1)
	value2 = binary.BigEndian.AppendUint64([]byte("value2"), 2)
	value3 = binary.BigEndian.AppendUint64([]byte("value3"), 3)
)

// TODO: improve generation of test values
func TestExtractSuffixes(t *testing.T) {
	tests := []struct {
		name    string
		storage map[string][]byte
		err     error
	}{
		{
			name:    "value too short to contain suffix",
			storage: map[string][]byte{keyStr: []byte("value")},
			err:     ErrValueTooShortForSuffix,
		},
		{
			name:    "single value with suffix",
			storage: map[string][]byte{keyStr: value},
		},
		{
			name: "multiple values with suffix",
			storage: map[string][]byte{
				key1Str: value1,
				key2Str: value2,
				key3Str: value3,
			},
		},
		{
			name: "mix of values with and without suffix",
			storage: map[string][]byte{
				key1Str: value1,
				key2Str: []byte("value2"),
			},
			err: ErrValueTooShortForSuffix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			unsuffixedStorage, _, err := ExtractSuffixes(tt.storage, 0, 0)
			if tt.err != nil {
				r.Equal(tt.err, err)
			} else {
				r.NoError(err)
				for k, v := range unsuffixedStorage {
					expectedValue, ok := tt.storage[k]
					r.True(ok)
					r.Equal(expectedValue[:len(expectedValue)-consts.Uint64Len], v)
				}
			}
		})
	}
}

func TestHotKeys(t *testing.T) {
	tests := []struct {
		name string

		storage     map[string][]byte
		blockHeight uint64
		epsilon     uint64

		expectedHotKeys map[string]uint16
		err             error
	}{
		{
			name:            "empty storage",
			storage:         map[string][]byte{},
			expectedHotKeys: map[string]uint16{},
		},
		{
			name:            "nonempty storage, but no hot keys",
			storage:         map[string][]byte{key1Str: value},
			expectedHotKeys: map[string]uint16{},
			blockHeight:     2,
			epsilon:         1,
		},
		{
			name:            "single hot key",
			storage:         map[string][]byte{key1Str: value1},
			expectedHotKeys: map[string]uint16{key1Str: 1},
			blockHeight:     1,
			epsilon:         1,
		},
		{
			name: "multiple hot keys",
			storage: map[string][]byte{
				key1Str: value1,
				key2Str: value2,
			},
			expectedHotKeys: map[string]uint16{
				key1Str: 1,
				key2Str: 2,
			},
			blockHeight: 3,
			epsilon:     2,
		},
		{
			name: "expected cold keys",
			storage: map[string][]byte{
				key1Str: value1,
				key2Str: value2,
			},
			expectedHotKeys: map[string]uint16{
				key2Str: 2,
			},
			blockHeight: 3,
			epsilon:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			_, hotKeys, err := ExtractSuffixes(tt.storage, tt.blockHeight, tt.epsilon)
			r.ErrorIs(err, tt.err)
			r.Equal(tt.expectedHotKeys, hotKeys)
		})
	}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package metadata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompatiblePrefixes(t *testing.T) {
	require := require.New(t)
	metadataManager := NewDefaultManager()

	require.False(
		HasConflictingPrefixes(
			metadataManager,
			[][]byte{
				{DefaultMinimumPrefix},
			},
		),
	)
}

func TestConflictingPrefixes(t *testing.T) {
	metadataManager := NewDefaultManager()

	tests := []struct {
		name            string
		metadataManager MetadataManager
		vmPrefixes      [][]byte
	}{
		{
			name:            "identical prefixes",
			metadataManager: metadataManager,
			vmPrefixes: [][]byte{
				metadataManager.HeightPrefix(),
			},
		},
		{
			name:            "metadataManager prefixes contains a prefix of one of the vm prefixes",
			metadataManager: metadataManager,
			vmPrefixes: [][]byte{
				{0x1, 0x1},
			},
		},
		{
			name: "vmPrefix contains a prefix of one of metadataManager prefixes",
			metadataManager: func() MetadataManager {
				return NewManager(
					[]byte{0x0, 0x1},
					[]byte{0x1, 0x1},
					[]byte{0x2, 0x1},
				)
			}(),
			vmPrefixes: [][]byte{
				{0x1},
			},
		},
		{
			name:            "vmPrefixes contains a duplicate",
			metadataManager: metadataManager,
			vmPrefixes: [][]byte{
				{DefaultMinimumPrefix},
				{DefaultMinimumPrefix},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			require.True(
				HasConflictingPrefixes(
					tt.metadataManager,
					tt.vmPrefixes,
				),
			)
		})
	}
}

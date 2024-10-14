// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package metadata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConflictingPrefixes(t *testing.T) {
	metadataManager := NewDefaultManager()

	tests := []struct {
		name            string
		metadataManager MetadataManager
		vmPrefixes      [][]byte
		hasConflict     bool
	}{
		{
			name:            "no conflicts",
			metadataManager: metadataManager,
			vmPrefixes: [][]byte{
				{DefaultMinimumPrefix},
			},
		},
		{
			name:            "identical prefixes",
			metadataManager: metadataManager,
			vmPrefixes: [][]byte{
				metadataManager.HeightPrefix(),
			},
			hasConflict: true,
		},
		{
			name:            "metadataManager prefixes contains a prefix of one of the vm prefixes",
			metadataManager: metadataManager,
			vmPrefixes: [][]byte{
				{0x1, 0x1},
			},
			hasConflict: true,
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
			hasConflict: true,
		},
		{
			name:            "vmPrefixes contains a duplicate",
			metadataManager: metadataManager,
			vmPrefixes: [][]byte{
				{DefaultMinimumPrefix},
				{DefaultMinimumPrefix},
			},
			hasConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			require.Equal(
				tt.hasConflict,
				HasConflictingPrefixes(
					tt.metadataManager,
					tt.vmPrefixes,
				),
			)
		})
	}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package metadata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConflictingPrefixes(t *testing.T) {
	require := require.New(t)

	metadataManager := NewDefaultManager()

	// Test no conflicts
	require.False(
		HasConflictingPrefixes(
			metadataManager,
			[][]byte{
				{DefaultMinimumPrefix},
			},
		),
	)
	// Test identical prefixes
	require.True(
		HasConflictingPrefixes(
			metadataManager,
			[][]byte{
				metadataManager.HeightPrefix(),
			},
		),
	)
	// Test that prefixes (from metadataManager) contains a prefix of one of the vm prefixes
	require.True(
		HasConflictingPrefixes(
			metadataManager,
			[][]byte{
				{0x1, 0x1},
			},
		),
	)

	customMetadataManager := NewManager(
		[]byte{0x0, 0x1},
		[]byte{0x1, 0x1},
		[]byte{0x2, 0x1},
	)

	// Test that vmPrefix contains prefix of one of metadataPrefixes
	require.True(
		HasConflictingPrefixes(
			customMetadataManager,
			[][]byte{
				{0x1},
			},
		),
	)
	// Test that vmPrefixes contains a duplicate
	require.True(
		HasConflictingPrefixes(
			metadataManager,
			[][]byte{
				{DefaultMinimumPrefix},
				{DefaultMinimumPrefix},
			},
		),
	)
}

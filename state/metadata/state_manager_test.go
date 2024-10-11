// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package metadata_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/state/metadata"
)

func TestConflictingPrefixes(t *testing.T) {
	require := require.New(t)

	metadataManager := metadata.NewDefaultManager()

	// Test no conflicts
	require.False(
		metadata.HasConflictingPrefixes(
			metadataManager,
			[][]byte{
				{metadata.DefaultMinimumPrefix},
			},
		),
	)
	// Test identical prefixes
	require.True(
		metadata.HasConflictingPrefixes(
			metadataManager,
			[][]byte{
				metadataManager.HeightPrefix(),
			},
		),
	)
	// Test that prefixes (from metadataManager) contains a prefix of one of the vm prefixes
	require.True(
		metadata.HasConflictingPrefixes(
			metadataManager,
			[][]byte{
				{0x1, 0x1},
			},
		),
	)

	customMetadataManager := metadata.NewManager(
		[]byte{0x0, 0x1},
		[]byte{0x1, 0x1},
		[]byte{0x2, 0x1},
	)

	// Test that vmPrefix contains prefix of one of metadataPrefixes
	require.True(
		metadata.HasConflictingPrefixes(
			customMetadataManager,
			[][]byte{
				{0x1},
			},
		),
	)
	// Test that vmPrefixes contains a duplicate
	require.True(
		metadata.HasConflictingPrefixes(
			metadataManager,
			[][]byte{
				{metadata.DefaultMinimumPrefix},
				{metadata.DefaultMinimumPrefix},
			},
		),
	)
}

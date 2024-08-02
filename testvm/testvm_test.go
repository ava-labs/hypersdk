// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testvm

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk/trace"
	"github.com/stretchr/testify/require"
)

func TestEnvSnapshot(t *testing.T) {
	require := require.New(t)

	config := TestConfig{
		// BlockProduction: BlockProduction{Type: Trigger},
		// MaxUnits:        [5]uint64{},
		// StateManager:    &storage.StateManager{},
		TracerConfig: trace.Config{Enabled: false},
		// Rules:        rules,
	}
	ctx := context.Background()
	vm, err := Init(ctx, config)
	require.NoError(err)
	require.Equal(uint64(1), vm.Height())
	snapId, err := vm.SnapshotSave()
	require.NoError(err)
	vm.SetHeight(10)
	require.Equal(uint64(10), vm.Height())
	err = vm.SnapshotRevert(snapId)
	require.NoError(err)
	require.Equal(uint64(1), vm.Height())
}

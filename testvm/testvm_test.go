// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testvm

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/trace"
	"github.com/stretchr/testify/require"
)

type StateManager struct{}

func (StateManager) SponsorStateKeys(addr codec.Address) state.Keys {
	return state.Keys{}
}

func (StateManager) CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) error {
	return nil
}

func (StateManager) Deduct(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error {
	return nil
}

func (StateManager) HeightKey() []byte {
	return nil
}

func (StateManager) TimestampKey() []byte {
	return nil
}

func (StateManager) FeeKey() []byte {
	return nil
}

func TestEnvSnapshot(t *testing.T) {
	require := require.New(t)

	config := TestConfig{
		StateManager: StateManager{},
		TracerConfig: trace.Config{Enabled: false},
	}
	ctx := context.Background()
	vm, err := NewTestVM(ctx, config)
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

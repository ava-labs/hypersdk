// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
)

var _ Action = (*mockAction)(nil)

type mockAction struct {
	Value uint16 `serialize:"true"`
}

// ComputeUnits implements Action.
func (*mockAction) ComputeUnits(Rules) uint64 {
	panic("unimplemented")
}

// Execute implements Action.
func (*mockAction) Execute(_ context.Context, _ Rules, _ state.Mutable, _ int64, _ codec.Address, _ ids.ID) (outputs [][]byte, err error) {
	panic("unimplemented")
}

// GetTypeID implements Action.
func (*mockAction) GetTypeID() uint8 {
	return 1
}

// StateKeys implements Action.
func (*mockAction) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	panic("unimplemented")
}

// StateKeysMaxChunks implements Action.
func (*mockAction) StateKeysMaxChunks() []uint16 {
	panic("unimplemented")
}

// ValidRange implements Action.
func (*mockAction) ValidRange(Rules) (start int64, end int64) {
	panic("unimplemented")
}

var _ HasSize = (*mockActionWithSizeFunc)(nil)

type mockActionWithSizeFunc struct {
	mockAction
}

func (*mockActionWithSizeFunc) Size() int {
	return 100000
}

var _ HasMarshal = (*mockActionWithMarshal)(nil)

type mockActionWithMarshal struct {
	mockAction
}

func (*mockActionWithMarshal) Marshal(p *codec.Packer) {
	p.PackFixedBytes([]byte{1, 2, 3, 4, 5})
}

func TestGetActionSize(t *testing.T) {
	require := require.New(t)

	actionNoSizeFunc := &mockAction{}
	actionWithSizeFunc := &mockActionWithSizeFunc{}

	size1, err := getActionSize(actionNoSizeFunc)
	require.NoError(err)
	require.Equal(2, size1)

	size2, err := getActionSize(actionWithSizeFunc)
	require.NoError(err)
	require.Equal(100000, size2)
}

func TestMarshalActionInto(t *testing.T) {
	require := require.New(t)

	actionNoMarshal := &mockAction{Value: 7}
	actionWithMarshal := &mockActionWithMarshal{}

	p1 := codec.NewWriter(0, consts.NetworkSizeLimit)
	err := marshalActionInto(actionNoMarshal, p1)
	require.NoError(err)
	require.Equal([]byte{0, 7}, p1.Bytes())

	p2 := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = marshalActionInto(actionWithMarshal, p2)
	require.NoError(err)
	require.Equal([]byte{1, 2, 3, 4, 5}, p2.Bytes())
}

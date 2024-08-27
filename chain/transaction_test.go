// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
)

var (
	_ chain.Action = (*mockTransferAction)(nil)
	_ chain.Action = (*action2)(nil)
)

type mockTransferAction struct {
	To    codec.Address `serialize:"true" json:"to"`
	Value uint64        `serialize:"true" json:"value"`
	Memo  []byte        `serialize:"true" json:"memo"`
}

type action2 struct {
	A uint64 `serialize:"true" json:"a"`
	B uint64 `serialize:"true" json:"b"`
}

func (*action2) ComputeUnits(chain.Rules) uint64 {
	panic("unimplemented")
}

func (*action2) Execute(_ context.Context, _ chain.Rules, _ state.Mutable, _ int64, _ codec.Address, _ ids.ID) (outputs [][]byte, err error) {
	panic("unimplemented")
}

func (*action2) GetTypeID() uint8 {
	return 222
}

func (*action2) Size() int {
	return 16
}

func (*action2) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	panic("unimplemented")
}

func (*action2) StateKeysMaxChunks() []uint16 {
	panic("unimplemented")
}

func (*action2) ValidRange(chain.Rules) (start int64, end int64) {
	panic("unimplemented")
}

func (*mockTransferAction) ComputeUnits(chain.Rules) uint64 {
	panic("ComputeUnits unimplemented")
}

func (*mockTransferAction) Execute(_ context.Context, _ chain.Rules, _ state.Mutable, _ int64, _ codec.Address, _ ids.ID) (outputs [][]byte, err error) {
	panic("Execute unimplemented")
}

func (*mockTransferAction) GetTypeID() uint8 {
	return 111
}

func (*mockTransferAction) Size() int {
	return 0
}

func (*mockTransferAction) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	panic("StateKeys unimplemented")
}

func (*mockTransferAction) StateKeysMaxChunks() []uint16 {
	panic("StateKeysMaxChunks unimplemented")
}

func (*mockTransferAction) ValidRange(chain.Rules) (start int64, end int64) {
	panic("ValidRange unimplemented")
}

func unmarshalTransfer(p *codec.Packer) (chain.Action, error) {
	var transfer mockTransferAction
	err := codec.AutoUnmarshalStruct(p, &transfer)
	return &transfer, err
}

func unmarshalAction2(p *codec.Packer) (chain.Action, error) {
	var action action2
	err := codec.AutoUnmarshalStruct(p, &action)
	return &action, err
}

func TestMarshalUnmarshal(t *testing.T) {
	require := require.New(t)

	tx := chain.Transaction{
		Base: &chain.Base{
			Timestamp: 1724315246000,
			ChainID:   [32]byte{1, 2, 3, 4, 5, 6, 7},
			MaxFee:    1234567,
		},
		Actions: []chain.Action{
			&mockTransferAction{
				To:    codec.Address{1, 2, 3, 4},
				Value: 4,
				Memo:  []byte("hello"),
			},
			&mockTransferAction{
				To:    codec.Address{4, 5, 6, 7},
				Value: 123,
				Memo:  []byte("world"),
			},
			&action2{
				A: 2,
				B: 4,
			},
		},
	}

	privBytes, err := hex.DecodeString("323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7")
	require.NoError(err)

	priv := ed25519.PrivateKey(privBytes)
	factory := auth.NewED25519Factory(priv)

	actionRegistry := codec.NewTypeParser[chain.Action]()
	authRegistry := codec.NewTypeParser[chain.Auth]()

	err = authRegistry.Register((&auth.ED25519{}).GetTypeID(), auth.UnmarshalED25519)
	require.NoError(err)
	err = actionRegistry.Register((&mockTransferAction{}).GetTypeID(), unmarshalTransfer)
	require.NoError(err)
	err = actionRegistry.Register((&action2{}).GetTypeID(), unmarshalAction2)
	require.NoError(err)

	signedTx, err := tx.Sign(factory, actionRegistry, authRegistry)
	require.NoError(err)

	require.Equal(len(signedTx.Actions), len(tx.Actions))
	for i, action := range signedTx.Actions {
		require.Equal(tx.Actions[i], action)
	}

	signedDigest, err := signedTx.Digest()
	require.NoError(err)
	txDigest, err := tx.Digest()
	require.NoError(err)

	require.Equal(signedDigest, txDigest)
	require.Len(signedDigest, 168)
}
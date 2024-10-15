// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
)

var (
	_ chain.Action = (*mockTransferAction)(nil)
	_ chain.Action = (*action2)(nil)
)

type abstractMockAction struct{}

func (*abstractMockAction) ComputeUnits(chain.Rules) uint64 {
	panic("unimplemented")
}

func (*abstractMockAction) Execute(_ context.Context, _ chain.Rules, _ state.Mutable, _ int64, _ codec.Address, _ ids.ID) (codec.Typed, error) {
	panic("unimplemented")
}

func (*abstractMockAction) StateKeys(_ codec.Address) state.Keys {
	panic("unimplemented")
}

func (*abstractMockAction) ValidRange(chain.Rules) (start int64, end int64) {
	panic("unimplemented")
}

type mockTransferAction struct {
	abstractMockAction
	To    codec.Address `serialize:"true" json:"to"`
	Value uint64        `serialize:"true" json:"value"`
	Memo  []byte        `serialize:"true" json:"memo"`
}

func (*mockTransferAction) GetTypeID() uint8 {
	return 111
}

type action2 struct {
	abstractMockAction
	A uint64 `serialize:"true" json:"a"`
	B uint64 `serialize:"true" json:"b"`
}

func (*action2) GetTypeID() uint8 {
	return 222
}

func unmarshalTransfer(p *codec.Packer) (chain.Action, error) {
	var transfer mockTransferAction
	err := codec.LinearCodec.UnmarshalFrom(p.Packer, &transfer)
	return &transfer, err
}

func unmarshalAction2(p *codec.Packer) (chain.Action, error) {
	var action action2
	err := codec.LinearCodec.UnmarshalFrom(p.Packer, &action)
	return &action, err
}

func TestJSONMarshalUnmarshal(t *testing.T) {
	require := require.New(t)

	txID := ids.GenerateTestID()
	pk, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	tx := &chain.Transaction{
		TransactionData: chain.TransactionData{
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
		},
		Auth: &auth.ED25519{
			Signer: pk.PublicKey(),
		},
	}
	tx.SetID(txID)

	b, err := json.Marshal(tx)
	require.NoError(err)

	actionCodec := codec.NewTypeParser[chain.Action]()
	authCodec := codec.NewTypeParser[chain.Auth]()

	err = actionCodec.Register(&mockTransferAction{}, unmarshalTransfer)
	require.NoError(err)
	err = actionCodec.Register(&action2{}, unmarshalAction2)
	require.NoError(err)
	err = authCodec.Register(&auth.ED25519{}, auth.UnmarshalED25519)
	require.NoError(err)
	parser := chaintest.NewParser(nil, actionCodec, authCodec, nil)

	var txout chain.Transaction
	err = txout.UnmarshalJSON(b, parser)
	require.NoError(err)
	require.Equal(*tx, txout)
}

// TestMarshalUnmarshal roughly validates that a transaction packs and unpacks correctly
func TestMarshalUnmarshal(t *testing.T) {
	require := require.New(t)

	tx := chain.TransactionData{
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

	priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)
	factory := auth.NewED25519Factory(priv)

	actionCodec := codec.NewTypeParser[chain.Action]()
	authCodec := codec.NewTypeParser[chain.Auth]()

	err = authCodec.Register(&auth.ED25519{}, auth.UnmarshalED25519)
	require.NoError(err)
	err = actionCodec.Register(&mockTransferAction{}, unmarshalTransfer)
	require.NoError(err)
	err = actionCodec.Register(&action2{}, unmarshalAction2)
	require.NoError(err)

	txBeforeSign := chain.TransactionData{
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
	// call UnsignedBytes so that the "unsignedBytes" field would get populated.
	_, err = txBeforeSign.UnsignedBytes()
	require.NoError(err)

	signedTx, err := tx.Sign(factory, actionCodec, authCodec)
	require.NoError(err)
	require.Equal(txBeforeSign, tx)
	require.NotNil(signedTx.Auth)
	require.Equal(len(signedTx.Actions), len(tx.Actions))
	for i, action := range signedTx.Actions {
		require.Equal(tx.Actions[i], action)
	}

	unsignedTxBytes, err := signedTx.UnsignedBytes()
	require.NoError(err)
	originalUnsignedTxBytes, err := tx.UnsignedBytes()
	require.NoError(err)

	require.Equal(unsignedTxBytes, originalUnsignedTxBytes)
	require.Len(unsignedTxBytes, 168)
}

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
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	_ chain.Action         = (*mockTransferAction)(nil)
	_ chain.Action         = (*action2)(nil)
	_ chain.Action         = (*action3)(nil)
	_ chain.BalanceHandler = (*mockBalanceHandler)(nil)
	_ chain.Auth           = (*abstractMockAuth)(nil)
	_ chain.Auth           = (*auth1)(nil)
)

type abstractMockAction struct{}

func (*abstractMockAction) ComputeUnits(chain.Rules) uint64 {
	panic("unimplemented")
}

func (*abstractMockAction) Execute(_ context.Context, _ chain.Rules, _ state.Mutable, _ int64, _ codec.Address, _ ids.ID) (codec.Typed, error) {
	panic("unimplemented")
}

func (*abstractMockAction) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
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

type action3 struct {
	abstractMockAction
	start int64
	end   int64
}

func (a *action3) ValidRange(_ chain.Rules) (int64, int64) {
	return a.start, a.end
}

func (*action3) GetTypeID() uint8 {
	return 3
}

type mockBalanceHandler struct{}

func (*mockBalanceHandler) AddBalance(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	panic("unimplemented")
}

func (*mockBalanceHandler) CanDeduct(_ context.Context, _ codec.Address, _ state.Immutable, _ uint64) error {
	panic("unimplemented")
}

func (*mockBalanceHandler) Deduct(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	panic("unimplemented")
}

func (*mockBalanceHandler) GetBalance(_ context.Context, _ codec.Address, _ state.Immutable) (uint64, error) {
	panic("unimplemented")
}

func (*mockBalanceHandler) SponsorStateKeys(_ codec.Address) state.Keys {
	panic("unimplemented")
}

type abstractMockAuth struct{}

func (*abstractMockAuth) Actor() codec.Address {
	panic("unimplemented")
}

func (*abstractMockAuth) ComputeUnits(_ chain.Rules) uint64 {
	panic("unimplemented")
}

func (*abstractMockAuth) GetTypeID() uint8 {
	panic("unimplemented")
}

func (*abstractMockAuth) Marshal(_ *codec.Packer) {
	panic("unimplemented")
}

func (*abstractMockAuth) Size() int {
	panic("unimplemented")
}

func (*abstractMockAuth) Sponsor() codec.Address {
	panic("unimplemented")
}

func (*abstractMockAuth) ValidRange(_ chain.Rules) (int64, int64) {
	panic("unimplemented")
}

func (*abstractMockAuth) Verify(_ context.Context, _ []byte) error {
	panic("unimplemented")
}

type auth1 struct {
	abstractMockAuth
	start int64
	end   int64
}

func (a *auth1) ValidRange(_ chain.Rules) (int64, int64) {
	return a.start, a.end
}

func TestJSONMarshalUnmarshal(t *testing.T) {
	require := require.New(t)

	txData := chain.TransactionData{
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

	err = actionCodec.Register(&mockTransferAction{}, unmarshalTransfer)
	require.NoError(err)
	err = actionCodec.Register(&action2{}, unmarshalAction2)
	require.NoError(err)
	err = authCodec.Register(&auth.ED25519{}, auth.UnmarshalED25519)
	require.NoError(err)

	signedTx, err := txData.Sign(factory)
	require.NoError(err)

	b, err := json.Marshal(signedTx)
	require.NoError(err)

	parser := chaintest.NewParser(nil, actionCodec, authCodec, nil)

	var txFromJSON chain.Transaction
	err = txFromJSON.UnmarshalJSON(b, parser)
	require.NoError(err)
	require.Equal(signedTx.Bytes(), txFromJSON.Bytes())
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

	// call UnsignedBytes so that the "unsignedBytes" field would get populated.
	txBeforeSignBytes, err := tx.UnsignedBytes()
	require.NoError(err)

	signedTx, err := tx.Sign(factory)
	require.NoError(err)
	unsignedTxAfterSignBytes, err := signedTx.TransactionData.UnsignedBytes()
	require.NoError(err)
	require.Equal(txBeforeSignBytes, unsignedTxAfterSignBytes)
	require.NotNil(signedTx.Auth)
	require.Equal(len(signedTx.Actions), len(tx.Actions))
	for i, action := range signedTx.Actions {
		require.Equal(tx.Actions[i], action)
	}
	writerPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = signedTx.Marshal(writerPacker)
	require.NoError(err)
	require.Equal(signedTx.GetID(), utils.ToID(writerPacker.Bytes()))
	require.Equal(signedTx.Bytes(), writerPacker.Bytes())

	unsignedTxBytes, err := signedTx.UnsignedBytes()
	require.NoError(err)
	originalUnsignedTxBytes, err := tx.UnsignedBytes()
	require.NoError(err)

	require.Equal(unsignedTxBytes, originalUnsignedTxBytes)
	require.Len(unsignedTxBytes, 168)
}

func TestSignRawActionBytesTx(t *testing.T) {
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

	signedTx, err := tx.Sign(factory)
	require.NoError(err)

	p := codec.NewWriter(0, consts.NetworkSizeLimit)
	require.NoError(signedTx.Actions.MarshalInto(p))
	actionsBytes := p.Bytes()
	rawSignedTxBytes, err := chain.SignRawActionBytesTx(tx.Base, actionsBytes, factory)
	require.NoError(err)
	require.Equal(signedTx.Bytes(), rawSignedTxBytes)
}

func TestPreExecute(t *testing.T) {
	// Test values
	testRules := genesis.NewDefaultRules()
	maxNumOfActions := 16
	differentChainID := ids.ID{1}

	tests := []struct {
		name      string
		tx        *chain.Transaction
		timestamp int64
		err       error
	}{
		{
			name: "base transaction timestamp misaligned",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{
						Timestamp: consts.MillisecondsPerSecond + 1,
					},
				},
			},
			err: chain.ErrMisalignedTime,
		},
		{
			name: "base transaction timestamp too late",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{
						Timestamp: consts.MillisecondsPerSecond,
					},
				},
			},
			timestamp: 2 * consts.MillisecondsPerSecond,
			err:       chain.ErrTimestampTooLate,
		},
		{
			name: "base transaction timestamp too early (61ms > 60ms)",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{
						Timestamp: testRules.GetValidityWindow() + consts.MillisecondsPerSecond,
					},
				},
			},
			err: chain.ErrTimestampTooEarly,
		},
		{
			name: "base transaction invalid chain id",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{
						ChainID:   differentChainID,
						Timestamp: consts.MillisecondsPerSecond,
					},
				},
			},
			timestamp: consts.MillisecondsPerSecond,
			err:       chain.ErrInvalidChainID,
		},
		{
			name: "transaction has too many actions (17 > 16)",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
					Actions: func() []chain.Action {
						actions := make([]chain.Action, maxNumOfActions+1)
						for i := 0; i < maxNumOfActions+1; i++ {
							actions = append(actions, &mockTransferAction{})
						}
						return actions
					}(),
				},
			},
			err: chain.ErrTooManyActions,
		},
		{
			name: "action timestamp too early (0ms < 3ms)",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
					Actions: []chain.Action{
						&action3{
							start: 3 * consts.MillisecondsPerSecond,
						},
					},
				},
			},
			err: chain.ErrActionNotActivated,
		},
		{
			name: "action timestamp too late (4ms > 3ms)",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{
						Timestamp: 4 * consts.MillisecondsPerSecond,
					},
					Actions: []chain.Action{
						&action3{
							end: 3 * consts.MillisecondsPerSecond,
						},
					},
				},
			},
			timestamp: 4 * consts.MillisecondsPerSecond,
			err:       chain.ErrActionNotActivated,
		},
		{
			name: "auth timestamp too early",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
				},
				Auth: &auth1{
					start: 1 * consts.MillisecondsPerSecond,
				},
			},
			err: chain.ErrAuthNotActivated,
		},
		{
			name: "auth timestamp too late",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{
						Timestamp: 2 * consts.MillisecondsPerSecond,
					},
				},
				Auth: &auth1{
					end: 1 * consts.MillisecondsPerSecond,
				},
			},
			timestamp: 2 * consts.MillisecondsPerSecond,
			err:       chain.ErrAuthNotActivated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

			r.ErrorIs(
				tt.tx.PreExecute(
					ctx,
					fees.NewManager([]byte{}),
					&mockBalanceHandler{},
					testRules,
					nil,
					tt.timestamp,
				),
				tt.err,
			)
		})
	}
}

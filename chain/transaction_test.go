// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"encoding/json"
	"errors"
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

	safemath "github.com/ava-labs/avalanchego/utils/math"
	externalfees "github.com/ava-labs/hypersdk/fees"
)

var (
	_ chain.Action         = (*mockTransferAction)(nil)
	_ chain.Action         = (*action2)(nil)
	_ chain.Action         = (*action3)(nil)
	_ chain.Action         = (*action4)(nil)
	_ chain.BalanceHandler = (*abstractMockBalanceHandler)(nil)
	_ chain.BalanceHandler = (*mockBalanceHandler1)(nil)
	_ chain.BalanceHandler = (*mockBalanceHandler2)(nil)
	_ chain.Auth           = (*abstractMockAuth)(nil)
	_ chain.Auth           = (*auth1)(nil)
)

var errMockInsufficientBalance = errors.New("mock insufficient balance error")

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

type action4 struct {
	abstractMockAction
	computeUnits uint64
}

func (a *action4) ComputeUnits(_ chain.Rules) uint64 {
	return a.computeUnits
}

func (*action4) GetTypeID() uint8 {
	return 4
}

func (*action4) ValidRange(_ chain.Rules) (int64, int64) {
	return -1, -1
}

func (*action4) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{}
}

type abstractMockBalanceHandler struct{}

func (*abstractMockBalanceHandler) AddBalance(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	panic("unimplemented")
}

func (*abstractMockBalanceHandler) CanDeduct(_ context.Context, _ codec.Address, _ state.Immutable, _ uint64) error {
	panic("unimplemented")
}

func (*abstractMockBalanceHandler) Deduct(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	panic("unimplemented")
}

func (*abstractMockBalanceHandler) GetBalance(_ context.Context, _ codec.Address, _ state.Immutable) (uint64, error) {
	panic("unimplemented")
}

func (*abstractMockBalanceHandler) SponsorStateKeys(_ codec.Address) state.Keys {
	panic("unimplemented")
}

type mockBalanceHandler1 struct {
	abstractMockBalanceHandler
}

func (*mockBalanceHandler1) SponsorStateKeys(_ codec.Address) state.Keys {
	return state.Keys{}
}

func (*mockBalanceHandler1) CanDeduct(_ context.Context, _ codec.Address, _ state.Immutable, _ uint64) error {
	return errMockInsufficientBalance
}

type mockBalanceHandler2 struct {
	abstractMockBalanceHandler
}

func (*mockBalanceHandler2) SponsorStateKeys(_ codec.Address) state.Keys {
	return state.Keys{}
}

func (*mockBalanceHandler2) CanDeduct(_ context.Context, _ codec.Address, _ state.Immutable, _ uint64) error {
	return nil
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

func (*auth1) ComputeUnits(_ chain.Rules) uint64 {
	return 0
}

func (*auth1) Actor() codec.Address {
	return codec.EmptyAddress
}

func (*auth1) Sponsor() codec.Address {
	return codec.EmptyAddress
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
	testRules := genesis.NewDefaultRules()
	maxNumOfActions := 16
	differentChainID := ids.ID{1}

	tests := []struct {
		name      string
		tx        *chain.Transaction
		timestamp int64
		err       error
		fm        *fees.Manager
		bh        chain.BalanceHandler
	}{
		{
			name: "valid test case",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
				},
				Auth: &auth1{},
			},
			bh: &mockBalanceHandler2{},
		},
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
			name: "base transaction timestamp too late (1ms < 2ms)",
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
			name: "action timestamp too early (0ms < 1ms)",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
					Actions: []chain.Action{
						&action3{
							start: consts.MillisecondsPerSecond,
						},
					},
				},
			},
			err: chain.ErrActionNotActivated,
		},
		{
			name: "action timestamp too late (2ms > 1ms)",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{
						Timestamp: 2 * consts.MillisecondsPerSecond,
					},
					Actions: []chain.Action{
						&action3{
							start: consts.MillisecondsPerSecond,
							end:   consts.MillisecondsPerSecond,
						},
					},
				},
			},
			timestamp: 2 * consts.MillisecondsPerSecond,
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
		{
			name: "fee overflow",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
					Actions: []chain.Action{
						&action4{
							computeUnits: 1,
						},
					},
				},
				Auth: &auth1{},
			},
			fm: func() *fees.Manager {
				fm := fees.NewManager([]byte{})
				for i := 0; i < externalfees.FeeDimensions; i++ {
					fm.SetUnitPrice(externalfees.Dimension(i), consts.MaxUint64)
				}
				return fm
			}(),
			bh:  &mockBalanceHandler1{},
			err: safemath.ErrOverflow,
		},
		{
			name: "insufficient balance",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
					Actions: []chain.Action{
						&action4{
							computeUnits: 1,
						},
					},
				},
				Auth: &auth1{},
			},
			bh:  &mockBalanceHandler1{},
			err: errMockInsufficientBalance,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

			if tt.fm == nil {
				tt.fm = fees.NewManager([]byte{})
			}
			if tt.bh == nil {
				tt.bh = &abstractMockBalanceHandler{}
			}

			r.ErrorIs(
				tt.tx.PreExecute(
					ctx,
					tt.fm,
					tt.bh,
					testRules,
					nil,
					tt.timestamp,
				),
				tt.err,
			)
		})
	}
}

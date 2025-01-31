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
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	safemath "github.com/ava-labs/avalanchego/utils/math"
	externalfees "github.com/ava-labs/hypersdk/fees"
)

var (
	_ chain.Action         = (*mockAction)(nil)
	_ chain.Action         = (*mockTransferAction)(nil)
	_ chain.Action         = (*action2)(nil)
	_ chain.BalanceHandler = (*mockBalanceHandler)(nil)
	_ chain.Auth           = (*mockAuth)(nil)
)

var errMockInsufficientBalance = errors.New("mock insufficient balance error")

type mockAction struct {
	start        int64
	end          int64
	computeUnits uint64
	typeID       uint8
	stateKeys    state.Keys
}

func (m *mockAction) ComputeUnits(chain.Rules) uint64 {
	return m.computeUnits
}

func (*mockAction) Execute(context.Context, chain.Rules, state.Mutable, int64, codec.Address, ids.ID) (codec.Typed, error) {
	panic("unimplemented")
}

func (m *mockAction) GetTypeID() uint8 {
	return m.typeID
}

func (m *mockAction) StateKeys(codec.Address, ids.ID) state.Keys {
	return m.stateKeys
}

func (m *mockAction) ValidRange(chain.Rules) (int64, int64) {
	return m.start, m.end
}

type mockTransferAction struct {
	mockAction
	To    codec.Address `serialize:"true" json:"to"`
	Value uint64        `serialize:"true" json:"value"`
	Memo  []byte        `serialize:"true" json:"memo"`
}

func (*mockTransferAction) GetTypeID() uint8 {
	return 111
}

type action2 struct {
	mockAction
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

type mockBalanceHandler struct {
	canDeductError error
}

func (*mockBalanceHandler) AddBalance(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	panic("unimplemented")
}

func (m *mockBalanceHandler) CanDeduct(_ context.Context, _ codec.Address, _ state.Immutable, _ uint64) error {
	return m.canDeductError
}

func (*mockBalanceHandler) Deduct(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	panic("unimplemented")
}

func (*mockBalanceHandler) GetBalance(_ context.Context, _ codec.Address, _ state.Immutable) (uint64, error) {
	panic("unimplemented")
}

func (*mockBalanceHandler) SponsorStateKeys(_ codec.Address) state.Keys {
	return state.Keys{}
}

type mockAuth struct {
	start        int64
	end          int64
	computeUnits uint64
	actor        codec.Address
	sponsor      codec.Address
	verifyError  error
}

func (m *mockAuth) Actor() codec.Address {
	return m.actor
}

func (m *mockAuth) ComputeUnits(chain.Rules) uint64 {
	return m.computeUnits
}

func (*mockAuth) GetTypeID() uint8 {
	panic("unimplemented")
}

func (*mockAuth) Marshal(*codec.Packer) {
	panic("unimplemented")
}

func (*mockAuth) Size() int {
	panic("unimplemented")
}

func (m *mockAuth) Sponsor() codec.Address {
	return m.sponsor
}

func (m *mockAuth) ValidRange(chain.Rules) (int64, int64) {
	return m.start, m.end
}

func (m *mockAuth) Verify(context.Context, []byte) error {
	return m.verifyError
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
				Auth: &mockAuth{},
			},
			bh: &mockBalanceHandler{},
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
			err: validitywindow.ErrMisalignedTime,
		},
		{
			name: "base transaction timestamp too far in future (61ms > 60ms)",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{
						Timestamp: testRules.GetValidityWindow() + consts.MillisecondsPerSecond,
					},
				},
			},
			err: validitywindow.ErrFutureTimestamp,
		},
		{
			name: "base transaction timestamp expired (1ms < 2ms)",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{
						Timestamp: consts.MillisecondsPerSecond,
					},
				},
			},
			timestamp: 2 * consts.MillisecondsPerSecond,
			err:       validitywindow.ErrTimestampExpired,
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
						actions := make([]chain.Action, testRules.MaxActionsPerTx+1)
						for i := 0; i < int(testRules.MaxActionsPerTx)+1; i++ {
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
						&mockAction{
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
						&mockAction{
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
				Auth: &mockAuth{
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
				Auth: &mockAuth{
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
						&mockAction{
							computeUnits: 1,
						},
					},
				},
				Auth: &mockAuth{},
			},
			fm: func() *fees.Manager {
				fm := fees.NewManager([]byte{})
				for i := 0; i < externalfees.FeeDimensions; i++ {
					fm.SetUnitPrice(externalfees.Dimension(i), consts.MaxUint64)
				}
				return fm
			}(),
			bh:  &mockBalanceHandler{},
			err: safemath.ErrOverflow,
		},
		{
			name: "insufficient balance",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
					Actions: []chain.Action{
						&mockAction{
							computeUnits: 1,
						},
					},
				},
				Auth: &mockAuth{},
			},
			bh: &mockBalanceHandler{
				canDeductError: errMockInsufficientBalance,
			},
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
				tt.bh = &mockBalanceHandler{}
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

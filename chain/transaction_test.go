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

var _ chain.BalanceHandler = (*mockBalanceHandler)(nil)

var errMockInsufficientBalance = errors.New("mock insufficient balance error")

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

func TestJSONMarshalUnmarshal(t *testing.T) {
	require := require.New(t)

	txData := chain.TransactionData{
		Base: &chain.Base{
			Timestamp: 1724315246000,
			ChainID:   [32]byte{1, 2, 3, 4, 5, 6, 7},
			MaxFee:    1234567,
		},
		Actions: []chain.Action{
			&chaintest.TestAction{
				NumComputeUnits: 1,
				ReadKeys:        [][]byte{{1, 2, 3, 4}},
			},
		},
	}
	priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)
	factory := auth.NewED25519Factory(priv)

	actionCodec := codec.NewTypeParser[chain.Action]()
	authCodec := codec.NewTypeParser[chain.Auth]()

	err = actionCodec.Register(&chaintest.TestAction{}, chaintest.UnmarshalTestAaction)
	require.NoError(err)
	require.NoError(err)
	err = authCodec.Register(&auth.ED25519{}, auth.UnmarshalED25519)
	require.NoError(err)
	parser := chain.NewTxTypeParser(actionCodec, authCodec)

	signedTx, err := txData.Sign(factory)
	require.NoError(err)

	b, err := json.Marshal(signedTx)
	require.NoError(err)

	txFromJSON := new(chain.Transaction)
	err = txFromJSON.UnmarshalJSON(b, parser)
	require.NoError(err, "failed to unmarshal tx JSON: %q", string(b))
	require.Equal(signedTx.Bytes(), txFromJSON.Bytes())
}

// TestMarshalUnmarshal roughly validates that a transaction packs and unpacks correctly
func TestMarshalUnmarshal(t *testing.T) {
	require := require.New(t)

	tx := chain.NewTxData(
		&chain.Base{
			Timestamp: 1724315246000,
			ChainID:   [32]byte{1, 2, 3, 4, 5, 6, 7},
			MaxFee:    1234567,
		},
		[]chain.Action{
			&chaintest.TestAction{
				NumComputeUnits: 2,
				ReadKeys:        [][]byte{{1, 2, 3, 4}},
				WriteKeys:       [][]byte{{5, 6, 7, 8}},
				WriteValues:     [][]byte{{9, 10, 11, 12}},
			},
			&chaintest.TestAction{
				NumComputeUnits: 1,
			},
		},
	)

	factory := &chaintest.TestAuthFactory{
		TestAuth: &chaintest.TestAuth{
			NumComputeUnits: 1,
			ActorAddress:    codec.EmptyAddress,
		},
	}

	parser := chaintest.NewTestParser()

	// call UnsignedBytes so that the "unsignedBytes" field would get populated.
	txBeforeSignBytes := tx.UnsignedBytes()

	signedTx, err := tx.Sign(factory)
	require.NoError(err)

	unsignedTxAfterSignBytes := signedTx.TransactionData.UnsignedBytes()
	require.Equal(txBeforeSignBytes, unsignedTxAfterSignBytes)
	require.NoError(signedTx.VerifyAuth(context.Background()))
	require.Equal(len(signedTx.Actions), len(tx.Actions))
	for i, action := range signedTx.Actions {
		require.Equal(tx.Actions[i], action)
	}

	signedTxBytes := signedTx.Bytes()
	require.Equal(signedTx.GetID(), utils.ToID(signedTxBytes))
	require.Equal(signedTx.Bytes(), signedTxBytes)

	unsignedTxBytes := signedTx.UnsignedBytes()
	originalUnsignedTxBytes := tx.UnsignedBytes()
	require.Equal(originalUnsignedTxBytes, unsignedTxBytes)

	parsedTx, err := chain.UnmarshalTx(signedTxBytes, parser)
	require.NoError(err)

	// We cannot do a simple equals check here because:
	// 1. AvalancheGo codec does not differentiate nil from empty slice, whereas equals does
	// 2. UnmarshalCanoto does not populate the canotoData size field
	// We check each field individually to confirm parsing produced the same result
	// We can remove this and use a simple equals check after resolving these issues:
	// 1. Fix equals check on unmarshalled canoto values https://github.com/StephenButtolph/canoto/issues/73
	// 2. Add dynamic serialization support to canoto https://github.com/StephenButtolph/canoto/issues/75
	require.Equal(signedTx.Bytes(), parsedTx.Bytes())
	require.Equal(signedTx.Base.MarshalCanoto(), parsedTx.Base.MarshalCanoto())
	require.Equal(len(signedTx.Actions), len(parsedTx.Actions))
	for i, action := range signedTx.Actions {
		require.Equal(action.Bytes(), parsedTx.Actions[i].Bytes())
	}
	require.Equal(signedTx.Auth, parsedTx.Auth)
}

func TestSignRawActionBytesTx(t *testing.T) {
	require := require.New(t)

	tx := chain.NewTxData(
		&chain.Base{
			Timestamp: 1724315246000,
			ChainID:   [32]byte{1, 2, 3, 4, 5, 6, 7},
			MaxFee:    1234567,
		},
		[]chain.Action{
			&chaintest.TestAction{
				NumComputeUnits: 2,
				ReadKeys:        [][]byte{{1, 2, 3, 4}},
				WriteKeys:       [][]byte{{5, 6, 7, 8}},
				WriteValues:     [][]byte{{9, 10, 11, 12}},
			},
			&chaintest.TestAction{
				NumComputeUnits: 1,
			},
		},
	)


	factory := &chaintest.TestAuthFactory{
		TestAuth: &chaintest.TestAuth{
			NumComputeUnits: 1,
			ActorAddress:    codec.EmptyAddress,
		},
	}

	parser := chaintest.NewTestParser()

	signedTx, err := tx.Sign(factory)
	require.NoError(err)

	actionsBytes := make([][]byte, 0, len(signedTx.Actions))
	for _, action := range signedTx.Actions {
		actionsBytes = append(actionsBytes, action.Bytes())
	}
	rawSignedTxBytes, err := chain.SignRawActionBytesTx(tx.Base, actionsBytes, factory)
	require.NoError(err)

	parseRawSignedTx, err := chain.UnmarshalTx(rawSignedTxBytes, parser)
	require.NoError(err)

	// TODO: fix canoto / AvalancheGo codec so we can perform a simple equals check
	require.Equal(signedTx.Base.MarshalCanoto(), parseRawSignedTx.Base.MarshalCanoto())
	require.Equal(len(signedTx.Actions), len(parseRawSignedTx.Actions))
	for i, action := range signedTx.Actions {
		require.Equal(action.Bytes(), parseRawSignedTx.Actions[i].Bytes())
	}
	require.Equal(signedTx.Auth, parseRawSignedTx.Auth)

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
				Auth: &chaintest.TestAuth{},
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
							actions = append(actions, &chaintest.TestAction{})
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
						&chaintest.TestAction{
							Start: consts.MillisecondsPerSecond,
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
						&chaintest.TestAction{
							Start: consts.MillisecondsPerSecond,
							End:   consts.MillisecondsPerSecond,
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
				Auth: &chaintest.TestAuth{
					Start: 1 * consts.MillisecondsPerSecond,
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
				Auth: &chaintest.TestAuth{
					End: 1 * consts.MillisecondsPerSecond,
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
						&chaintest.TestAction{
							NumComputeUnits: 1,
						},
					},
				},
				Auth: &chaintest.TestAuth{},
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
						&chaintest.TestAction{
							NumComputeUnits: 1,
						},
					},
				},
				Auth: &chaintest.TestAuth{},
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

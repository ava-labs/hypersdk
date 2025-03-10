// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/state/balance"
	"github.com/ava-labs/hypersdk/utils"

	safemath "github.com/ava-labs/avalanchego/utils/math"
	externalfees "github.com/ava-labs/hypersdk/fees"
)

const signedTxHex = "0a3208b0bbcac99732122001020304050607000000000000000000000000000000000000000000000000001987d612000000000012360000000000000000010000000000000000000000000000000000000000000000000000000000ffffffffffffffffffffffffffffffff1a5c00000000000000000101020300000000000000000000000000000000000000000000000000000000000001020300000000000000000000000000000000000000000000000000000000000000ffffffffffffffffffffffffffffffff"

var preSignedTxBytes []byte

func init() {
	txBytes, err := hex.DecodeString(signedTxHex)
	if err != nil {
		panic(err)
	}
	preSignedTxBytes = txBytes
}

func TestTransactionJSON(t *testing.T) {
	r := require.New(t)

	txData := chain.NewTxData(
		chain.Base{
			Timestamp: 1724315246000,
			ChainID:   [32]byte{1, 2, 3, 4, 5, 6, 7},
			MaxFee:    1234567,
		},
		[]chain.Action{
			chaintest.NewDummyTestAction(),
		},
	)
	authFactory := &chaintest.TestAuthFactory{
		TestAuth: chaintest.NewDummyTestAuth(),
	}
	parser := chaintest.NewTestParser()

	signedTx, err := txData.Sign(authFactory)
	r.NoError(err)

	b, err := json.Marshal(signedTx)
	r.NoError(err)

	txFromJSON := new(chain.Transaction)
	err = txFromJSON.UnmarshalJSON(b, parser)
	r.NoError(err, "failed to unmarshal tx JSON: %q", string(b))
	equalTx(r, signedTx, txFromJSON)
}

func TestSignTransaction(t *testing.T) {
	r := require.New(t)

	txData := chain.NewTxData(
		chain.Base{
			Timestamp: 1724315246000,
			ChainID:   [32]byte{1, 2, 3, 4, 5, 6, 7},
			MaxFee:    1234567,
		},
		[]chain.Action{
			chaintest.NewDummyTestAction(),
			chaintest.NewDummyTestAction(),
		},
	)
	authFactory := &chaintest.TestAuthFactory{
		TestAuth: chaintest.NewDummyTestAuth(),
	}
	parser := chaintest.NewTestParser()

	txBeforeSignBytes := utils.CopyBytes(txData.UnsignedBytes())
	signedTx, err := txData.Sign(authFactory)
	r.NoError(err)

	unsignedTxAfterSignBytes := signedTx.TransactionData.UnsignedBytes()
	r.Equal(txBeforeSignBytes, unsignedTxAfterSignBytes, "signed unsigned bytes matches original unsigned tx data")
	r.NoError(signedTx.VerifyAuth(context.Background()))
	equalTxData(r, txData, signedTx.TransactionData, "signed tx data matches original tx data")

	signedTxBytes := signedTx.Bytes()
	r.Equal(signedTx.GetID(), utils.ToID(signedTxBytes), "signed txID matches expected txID")

	unsignedTxBytes := signedTx.UnsignedBytes()
	originalUnsignedTxBytes := txData.UnsignedBytes()
	r.Equal(originalUnsignedTxBytes, unsignedTxBytes)

	parsedTx, err := chain.UnmarshalTx(signedTxBytes, parser)
	r.NoError(err)

	equalTx(r, signedTx, parsedTx)
}

func TestSignRawActionBytesTx(t *testing.T) {
	require := require.New(t)

	txData := chain.NewTxData(
		chain.Base{
			Timestamp: 1724315246000,
			ChainID:   [32]byte{1, 2, 3, 4, 5, 6, 7},
			MaxFee:    1234567,
		},
		[]chain.Action{
			chaintest.NewDummyTestAction(),
		},
	)

	factory := &chaintest.TestAuthFactory{
		TestAuth: chaintest.NewDummyTestAuth(),
	}

	parser := chaintest.NewTestParser()

	signedTx, err := txData.Sign(factory)
	require.NoError(err)

	actionsBytes := make([][]byte, 0, len(signedTx.Actions))
	for _, action := range signedTx.Actions {
		actionsBytes = append(actionsBytes, action.Bytes())
	}
	rawSignedTxBytes, err := chain.SignRawActionBytesTx(txData.Base, actionsBytes, factory)
	require.NoError(err)

	parseRawSignedTx, err := chain.UnmarshalTx(rawSignedTxBytes, parser)
	require.NoError(err)

	equalTx(require, signedTx, parseRawSignedTx)
}

func TestUnmarshalTx(t *testing.T) {
	r := require.New(t)

	txData := chain.NewTxData(
		chain.Base{
			Timestamp: 1724315246000,
			ChainID:   ids.ID{1, 2, 3, 4, 5, 6, 7},
			MaxFee:    1234567,
		},
		[]chain.Action{
			chaintest.NewDummyTestAction(),
		},
	)
	authFactory := &chaintest.TestAuthFactory{
		TestAuth: chaintest.NewDummyTestAuth(),
	}
	parser := chaintest.NewTestParser()

	signedTx, err := txData.Sign(authFactory)
	r.NoError(err)

	signedTxBytes := signedTx.Bytes()
	parsedTx, err := chain.UnmarshalTx(signedTxBytes, parser)
	r.NoError(err)

	equalTx(r, signedTx, parsedTx)
	r.Equal(preSignedTxBytes, signedTxBytes, "expected %x, actual %x", preSignedTxBytes, signedTxBytes)
}

// go test -benchmem -run=^$ -bench ^BenchmarkUnmarshalTx$ github.com/ava-labs/hypersdk/chain -timeout=15s
//
// goos: darwin
// goarch: arm64
// pkg: github.com/ava-labs/hypersdk/chain
// BenchmarkUnmarshalTx-12    	  452396	      3008 ns/op	    2336 B/op	      20 allocs/op
// PASS
// ok  	github.com/ava-labs/hypersdk/chain	1.748s
func BenchmarkUnmarshalTx(b *testing.B) {
	parser := chaintest.NewTestParser()
	r := require.New(b)
	b.ResetTimer()
	for range b.N {
		tx, err := chain.UnmarshalTx(preSignedTxBytes, parser)
		r.NoError(err)
		r.NotNil(tx)
	}
}

func TestEstimateUnits(t *testing.T) {
	r := require.New(t)

	txData := chain.NewTxData(
		chain.Base{
			Timestamp: 1724315246000,
			ChainID:   ids.ID{1, 2, 3, 4, 5, 6, 7},
			MaxFee:    1234567,
		},
		[]chain.Action{
			chaintest.NewDummyTestAction(),
		},
	)
	authFactory := &chaintest.TestAuthFactory{
		TestAuth: chaintest.NewDummyTestAuth(),
	}
	signedTx, err := txData.Sign(authFactory)
	r.NoError(err)

	rules := genesis.NewDefaultRules()
	balanceHandler := balance.NewPrefixBalanceHandler([]byte{0})

	estimatedUnits, err := chain.EstimateUnits(rules, txData.Actions, authFactory)
	r.NoError(err)

	actualUnits, err := signedTx.Units(balanceHandler, rules)
	r.NoError(err)
	for i := range estimatedUnits {
		r.LessOrEqual(actualUnits[i], estimatedUnits[i])
	}
}

type immutableStateStub struct {
	GetValueF func(context.Context, []byte) ([]byte, error)
}

func (iss *immutableStateStub) GetValue(ctx context.Context, key []byte) ([]byte, error) {
	return iss.GetValueF(ctx, key)
}

func TestPreExecute(t *testing.T) {
	testRules := genesis.NewDefaultRules()
	differentChainID := ids.ID{1}

	im := &immutableStateStub{
		GetValueF: func(context.Context, []byte) ([]byte, error) {
			return binary.BigEndian.AppendUint64(nil, 0), nil
		},
	}

	tests := []struct {
		name      string
		tx        *chain.Transaction
		timestamp int64
		err       error
		fm        *fees.Manager
	}{
		{
			name: "valid test case",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: chain.Base{},
				},
				Auth: chaintest.NewDummyTestAuth(),
			},
		},
		{
			name: "base transaction timestamp misaligned",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: chain.Base{
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
					Base: chain.Base{
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
					Base: chain.Base{
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
					Base: chain.Base{
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
					Base: chain.Base{},
					Actions: func() []chain.Action {
						actions := make([]chain.Action, testRules.MaxActionsPerTx+1)
						for i := 0; i < int(testRules.MaxActionsPerTx)+1; i++ {
							actions = append(actions, chaintest.NewDummyTestAction())
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
					Base: chain.Base{},
					Actions: []chain.Action{
						&chaintest.TestAction{
							Start: consts.MillisecondsPerSecond,
							End:   -1,
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
					Base: chain.Base{
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
					Base: chain.Base{},
				},
				Auth: &chaintest.TestAuth{
					Start: 1 * consts.MillisecondsPerSecond,
					End:   -1,
				},
			},
			err: chain.ErrAuthNotActivated,
		},
		{
			name: "auth timestamp too late",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: chain.Base{
						Timestamp: 2 * consts.MillisecondsPerSecond,
					},
				},
				Auth: &chaintest.TestAuth{
					Start: -1,
					End:   1 * consts.MillisecondsPerSecond,
				},
			},
			timestamp: 2 * consts.MillisecondsPerSecond,
			err:       chain.ErrAuthNotActivated,
		},
		{
			name: "fee overflow",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: chain.Base{},
					Actions: []chain.Action{
						chaintest.NewDummyTestAction(),
					},
				},
				Auth: chaintest.NewDummyTestAuth(),
			},
			fm: func() *fees.Manager {
				fm := fees.NewManager([]byte{})
				for i := 0; i < externalfees.FeeDimensions; i++ {
					fm.SetUnitPrice(externalfees.Dimension(i), consts.MaxUint64)
				}
				return fm
			}(),
			err: safemath.ErrOverflow,
		},
		{
			name: "insufficient balance",
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: chain.Base{},
					Actions: []chain.Action{
						chaintest.NewDummyTestAction(),
					},
				},
				Auth: chaintest.NewDummyTestAuth(),
			},
			fm: func() *fees.Manager {
				fm := fees.NewManager([]byte{})
				for i := 0; i < externalfees.FeeDimensions; i++ {
					fm.SetUnitPrice(externalfees.Dimension(i), 1)
				}
				return fm
			}(),
			err: balance.ErrInsufficientBalance,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

			if tt.fm == nil {
				tt.fm = fees.NewManager([]byte{})
			}
			r.ErrorIs(
				tt.tx.PreExecute(
					ctx,
					tt.fm,
					balance.NewPrefixBalanceHandler([]byte{0}),
					testRules,
					im,
					tt.timestamp,
				),
				tt.err,
			)
		})
	}
}

// equalTx confirms that the expected and actual transactions are equal
//
// We cannot do a simple equals check here because:
// 1. AvalancheGo codec does not differentiate nil from empty slice, whereas equals does
// 2. UnmarshalCanoto does not populate the canotoData size field
// We check each field individually to confirm parsing produced the same result
// We can remove this and use a simple equals check after resolving these issues:
// 1. Fix equals check on unmarshalled canoto values https://github.com/StephenButtolph/canoto/issues/73
// 2. Add dynamic serialization support to canoto https://github.com/StephenButtolph/canoto/issues/75
//
// TODO: replace this at the call site with a simple equals check after fixing the above issues
func equalTx(r *require.Assertions, expected *chain.Transaction, actual *chain.Transaction) {
	equalTxData(r, expected.TransactionData, actual.TransactionData)
	r.Equal(expected.Auth, actual.Auth)
	r.Equal(expected.Bytes(), actual.Bytes())
}

func equalTxData(r *require.Assertions, expected chain.TransactionData, actual chain.TransactionData, msgAndArgs ...interface{}) {
	r.Equal(expected.Base.MarshalCanoto(), actual.Base.MarshalCanoto(), msgAndArgs...)
	r.Equal(len(expected.Actions), len(actual.Actions), msgAndArgs...)
	for i, action := range expected.Actions {
		msgAndArgs = append(msgAndArgs, "index", i)
		r.Equal(action.Bytes(), actual.Actions[i].Bytes(), msgAndArgs...)
	}
	r.Equal(expected.UnsignedBytes(), actual.UnsignedBytes(), msgAndArgs...)
}

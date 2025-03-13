// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/balance"
	"github.com/ava-labs/hypersdk/utils"

	safemath "github.com/ava-labs/avalanchego/utils/math"
	externalfees "github.com/ava-labs/hypersdk/fees"
)

var (
	//go:embed chaintest/testdata/signedTransaction.hex
	signedTxFS embed.FS

	//go:embed chaintest/testdata/signedTransaction.json
	signedTxJSONFS embed.FS
)

var (
	_                chain.BalanceHandler = (*mockBalanceHandler)(nil)
	preSignedTxBytes []byte
	signedTxJSON     []byte

	errMockInsufficientBalance = errors.New("mock insufficient balance error")
)

func init() {
	updateReferenceTxData()

	signedTx, readHexErr := signedTxFS.ReadFile("chaintest/testdata/signedTransaction.hex")
	signedTxJSONRaw, readJSONErr := signedTxJSONFS.ReadFile("chaintest/testdata/signedTransaction.json")
	err := errors.Join(readHexErr, readJSONErr)
	if err != nil {
		panic(err)
	}
	signedTxJSON = signedTxJSONRaw
	signedTxHex := strings.TrimSpace(string(signedTx))
	txBytes, err := hex.DecodeString(signedTxHex)
	if err != nil {
		panic(err)
	}
	preSignedTxBytes = txBytes
}

type mockBalanceHandler struct {
	canDeductError error
	deductError    error
}

func (*mockBalanceHandler) AddBalance(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	panic("unimplemented")
}

func (m *mockBalanceHandler) CanDeduct(_ context.Context, _ codec.Address, _ state.Immutable, _ uint64) error {
	return m.canDeductError
}

func (m *mockBalanceHandler) Deduct(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	return m.deductError
}

func (*mockBalanceHandler) GetBalance(_ context.Context, _ codec.Address, _ state.Immutable) (uint64, error) {
	panic("unimplemented")
}

func (*mockBalanceHandler) SponsorStateKeys(_ codec.Address) state.Keys {
	return state.Keys{}
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

	factory := &chaintest.TestAuthFactory{
		TestAuth: chaintest.NewDummyTestAuth(),
	}

	parser := chaintest.NewTestParser()

	signedTx, err := txData.Sign(factory)
	r.NoError(err)

	actionsBytes := make([][]byte, 0, len(signedTx.Actions))
	for _, action := range signedTx.Actions {
		actionsBytes = append(actionsBytes, action.Bytes())
	}
	rawSignedTxBytes, err := chain.SignRawActionBytesTx(txData.Base, actionsBytes, factory)
	r.NoError(err)

	parseRawSignedTx, err := chain.UnmarshalTx(rawSignedTxBytes, parser)
	r.NoError(err)

	equalTx(r, signedTx, parseRawSignedTx)
}

func TestUnmarshalTx(t *testing.T) {
	r := require.New(t)

	txFromJSON := new(chain.Transaction)
	parser := chaintest.NewTestParser()
	err := txFromJSON.UnmarshalJSON(signedTxJSON, parser)
	r.NoError(err)

	signedTxBytes := txFromJSON.Bytes()
	parsedTx, err := chain.UnmarshalTx(signedTxBytes, parser)
	r.NoError(err)

	equalTx(r, txFromJSON, parsedTx)
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
					Base: chain.Base{},
				},
				Auth: chaintest.NewDummyTestAuth(),
			},
			bh: &mockBalanceHandler{},
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
			bh:  &mockBalanceHandler{},
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

// updateReferenceBlockData regenerates the reference hex and JSON files with the current implementation
// Only runs when UPDATE_TEST_DATA=1 is set eg: UPDATE_TEST_DATA=1 go test ./chain/...
func updateReferenceTxData() {
	// Only run when explicitly enabled
	if os.Getenv("UPDATE_TEST_DATA") != "1" {
		return
	}

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
	if err != nil {
		panic(err)
	}
	b, err := json.Marshal(signedTx)
	if err != nil {
		panic(err)
	}
	singedTxBytes := make([]byte, hex.EncodedLen(len(signedTx.Bytes())))
	hex.Encode(singedTxBytes, signedTx.Bytes())
	err = errors.Join(os.WriteFile("chaintest/testdata/signedTransaction.json", b, 0o600), os.WriteFile("chaintest/testdata/signedTransaction.hex", singedTxBytes, 0o600))
	if err != nil {
		panic(err)
	}
}

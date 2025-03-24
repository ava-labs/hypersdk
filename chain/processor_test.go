// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/validitywindow/validitywindowtest"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/balance"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/utils"
)

var errMockVerifyExpiryReplayProtection = errors.New("mock validity window error")

func TestProcessorExecute(t *testing.T) {
	testRules := genesis.NewDefaultRules()

	testMetadataManager := metadata.NewDefaultManager()
	feeKey := string(chain.FeeKey(testMetadataManager.FeePrefix()))
	heightKey := string(chain.HeightKey(testMetadataManager.HeightPrefix()))
	timestampKey := string(chain.TimestampKey(testMetadataManager.TimestampPrefix()))
	pk, err := ed25519.GeneratePrivateKey()
	require.NoError(t, err)
	balanceHandler := balance.NewPrefixBalanceHandler([]byte{0})

	tests := []struct {
		name           string
		validityWindow chain.ValidityWindow
		isNormalOp     bool
		newViewF       func(*require.Assertions) merkledb.View
		newBlockF      func(*require.Assertions, ids.ID) *chain.StatelessBlock
		expectedErr    error
	}{
		{
			name:           "valid test case",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
		},
		{
			name:           "block timestamp too late",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					time.Now().Add(2*chain.FutureBound).UnixMilli(),
					0,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			expectedErr: chain.ErrTimestampTooLate,
		},
		{
			name:           "failed to get parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrFailedToFetchParentHeight,
		},
		{
			name:           "failed to parse parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    {},
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrFailedToParseParentHeight,
		},
		{
			name:           "block height is not one more than parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					2,
					nil,
					parentRoot,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrInvalidBlockHeight,
		},
		{
			name:           "failed to get timestamp",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:    {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrFailedToFetchParentTimestamp,
		},
		{
			name:           "failed to parse timestamp",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: {},
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrFailedToParseParentTimestamp,
		},
		{
			name:           "non-empty block - timestamp less than parent timestamp with gap",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				tx, err := chain.NewTransaction(
					chain.Base{
						Timestamp: utils.UnixRMilli(
							testRules.GetMinEmptyBlockGap(),
							testRules.GetValidityWindow(),
						),
					},
					[]chain.Action{},
					chaintest.NewDummyTestAuth(),
				)
				r.NoError(err)

				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinBlockGap()-1,
					1,
					[]*chain.Transaction{tx},
					parentRoot,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrTimestampTooEarly,
		},
		{
			name:           "empty block - timestamp less than parent timestamp with gap",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap()-1,
					1,
					nil,
					parentRoot,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrTimestampTooEarlyEmptyBlock,
		},
		{
			name:           "failed to get fee",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrFailedToFetchParentFee,
		},
		{
			name: "fails replay protection",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{
				OnVerifyExpiryReplayProtection: func(context.Context, validitywindow.ExecutionBlock[*chain.Transaction]) error {
					return errMockVerifyExpiryReplayProtection
				},
			},
			isNormalOp: true,
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: errMockVerifyExpiryReplayProtection,
		},
		{
			name:           "failed to execute txs",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				tx, err := chain.NewTransaction(
					chain.Base{
						Timestamp: utils.UnixRMilli(
							testRules.GetMinEmptyBlockGap(),
							testRules.GetValidityWindow(),
						),
					},
					[]chain.Action{
						&chaintest.TestAction{
							NumComputeUnits: 1,
							Start:           -1,
							End:             -1,
							SpecifiedStateKeys: []string{
								"",
							},
							SpecifiedStateKeyPermissions: []state.Permissions{
								state.None,
							},
						},
					},
					chaintest.NewDummyTestAuth(),
				)
				r.NoError(err)

				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinBlockGap(),
					1,
					[]*chain.Transaction{tx},
					parentRoot,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrInvalidKeyValue,
		},
		{
			name:           "state root mismatch",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, _ ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					ids.GenerateTestID(),
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrStateRootMismatch,
		},
		{
			name:           "invalid transaction signature",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				auth := auth.ED25519{
					Signer: pk.PublicKey(),
				}
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
					string(balanceHandler.BalanceKey(auth.Sponsor())): binary.BigEndian.AppendUint64(nil, math.MaxUint64),
				})

				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				tx, err := chain.NewTransaction(
					chain.Base{
						Timestamp: utils.UnixRMilli(
							testRules.GetMinEmptyBlockGap(),
							testRules.GetValidityWindow(),
						),
					},
					[]chain.Action{},
					&auth.ED25519{
						Signer: pk.PublicKey(),
					},
				)
				r.NoError(err)

				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinBlockGap(),
					1,
					[]*chain.Transaction{tx},
					parentRoot,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: crypto.ErrInvalidSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

			metrics, err := chain.NewMetrics(prometheus.NewRegistry())
			r.NoError(err)

			processor := chain.NewProcessor(
				trace.Noop,
				&logging.NoLog{},
				&genesis.ImmutableRuleFactory{Rules: testRules},
				workers.NewSerial(),
				chaintest.NewDummyTestAuthEngines(),
				testMetadataManager,
				balanceHandler,
				tt.validityWindow,
				metrics,
				chain.NewDefaultConfig(),
			)

			view := tt.newViewF(r)
			root, err := view.GetMerkleRoot(ctx)
			r.NoError(err)

			block := tt.newBlockF(r, root)

			_, err = processor.Execute(
				ctx,
				view,
				chain.NewExecutionBlock(block),
				tt.isNormalOp,
			)
			r.ErrorIs(err, tt.expectedErr)
		})
	}
}

func BenchmarkExecuteBlocks(b *testing.B) {
	benchmarks := []struct {
		name                 string
		blockBenchmarkHelper chaintest.BlockBenchmarkHelper
		numOfTxsPerBlock     uint64
	}{
		{
			name:                 "empty blocks",
			blockBenchmarkHelper: chaintest.NoopBlockBenchmarkHelper,
		},
		{
			name:                 "blocks with txs that do not have conflicting state keys",
			blockBenchmarkHelper: parallelTxsBlockBenchmarkHelper,
			numOfTxsPerBlock:     16,
		},
		{
			name:                 "blocks with txs that all touch the same state key",
			blockBenchmarkHelper: serialTxsBlockBenchmarkHelper,
			numOfTxsPerBlock:     16,
		},
		{
			name:                 "blocks with txs whose state keys are zipf distributed",
			blockBenchmarkHelper: zipfTxsBlockBenchmarkHelper,
			numOfTxsPerBlock:     16,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			benchmark := &chaintest.BlockBenchmark{
				MetadataManager:      metadata.NewDefaultManager(),
				BalanceHandler:       &mockBalanceHandler{},
				AuthEngines:          auth.DefaultEngines(),
				RuleFactory:          &genesis.ImmutableRuleFactory{Rules: genesis.NewDefaultRules()},
				BlockBenchmarkHelper: bm.blockBenchmarkHelper,
				Config: chain.Config{
					TargetBuildDuration:       100 * time.Millisecond,
					TransactionExecutionCores: 4,
					StateFetchConcurrency:     4,
					TargetTxsSize:             1.5 * units.MiB,
				},
				NumOfBlocks:      1_000,
				NumOfTxsPerBlock: bm.numOfTxsPerBlock,
			}
			benchmark.Run(context.Background(), b)
		})
	}
}

func parallelTxsBlockBenchmarkHelper(numOfTxs uint64) (genesis.Genesis, chaintest.TxListGenerator, error) {
	factories, genesis, err := createGenesis(numOfTxs, 1_000_000)
	if err != nil {
		return nil, nil, err
	}

	// Generate kv-pairs for testing
	databaseKeys := make([]string, numOfTxs)
	values := make([][]byte, numOfTxs)

	for i := range numOfTxs {
		key := string(binary.BigEndian.AppendUint16(
			binary.BigEndian.AppendUint64(nil, i),
			1,
		))
		databaseKeys[i] = key
		values[i] = binary.BigEndian.AppendUint64(nil, i)
	}

	nonce := uint64(0)

	txListGenerator := func(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
		txs := make([]*chain.Transaction, numOfTxsPerBlock)
		for i := range numOfTxsPerBlock {
			action := &chaintest.TestAction{
				Nonce:                        nonce,
				SpecifiedStateKeys:           []string{databaseKeys[i]},
				SpecifiedStateKeyPermissions: []state.Permissions{state.Write},
				WriteKeys:                    [][]byte{[]byte(databaseKeys[i])},
				WriteValues:                  [][]byte{values[i]},
				Start:                        -1,
				End:                          -1,
			}

			nonce++

			tx, err := txGenerator([]chain.Action{action}, factories[i])
			if err != nil {
				return nil, err
			}

			txs[i] = tx
		}
		return txs, nil
	}

	return genesis, txListGenerator, nil
}

func serialTxsBlockBenchmarkHelper(numOfTxsPerBlock uint64) (genesis.Genesis, chaintest.TxListGenerator, error) {
	factories, genesis, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, nil, err
	}

	k := string(binary.BigEndian.AppendUint16(nil, 1))
	v := binary.BigEndian.AppendUint64(nil, 1)

	nonce := uint64(0)

	txListGenerator := func(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
		txs := make([]*chain.Transaction, numOfTxsPerBlock)
		for i := range numOfTxsPerBlock {
			action := &chaintest.TestAction{
				Nonce:                        nonce,
				SpecifiedStateKeys:           []string{k},
				SpecifiedStateKeyPermissions: []state.Permissions{state.Write},
				WriteKeys:                    [][]byte{v},
				Start:                        -1,
				End:                          -1,
			}

			nonce++

			tx, err := txGenerator([]chain.Action{action}, factories[i])
			if err != nil {
				return nil, err
			}

			txs[i] = tx
		}
		return txs, nil
	}

	return genesis, txListGenerator, nil
}

func zipfTxsBlockBenchmarkHelper(numOfTxsPerBlock uint64) (genesis.Genesis, chaintest.TxListGenerator, error) {
	factories, genesis, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, nil, err
	}

	// Generate kv-pairs for testing
	databaseKeys := make([]string, numOfTxsPerBlock)
	values := make([][]byte, numOfTxsPerBlock)

	for i := range numOfTxsPerBlock {
		key := string(binary.BigEndian.AppendUint16(
			binary.BigEndian.AppendUint64(nil, i),
			1,
		))
		databaseKeys[i] = key
		values[i] = binary.BigEndian.AppendUint64(nil, i)
	}

	nonce := uint64(0)

	zipfSeed := rand.New(rand.NewSource(0)) //nolint:gosec
	sZipf := 1.01
	vZipf := 2.7
	zipfGen := rand.NewZipf(zipfSeed, sZipf, vZipf, numOfTxsPerBlock-1)

	txListGenerator := func(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
		txs := make([]*chain.Transaction, numOfTxsPerBlock)
		for i := range numOfTxsPerBlock {
			index := zipfGen.Uint64()
			action := &chaintest.TestAction{
				Nonce:                        nonce,
				SpecifiedStateKeys:           []string{databaseKeys[index]},
				SpecifiedStateKeyPermissions: []state.Permissions{state.Write},
				WriteKeys:                    [][]byte{[]byte(databaseKeys[index])},
				WriteValues:                  [][]byte{values[index]},
				Start:                        -1,
				End:                          -1,
			}

			nonce++

			tx, err := txGenerator([]chain.Action{action}, factories[i])
			if err != nil {
				return nil, err
			}

			txs[i] = tx
		}
		return txs, nil
	}

	return genesis, txListGenerator, nil
}

func createGenesis(numOfFactories uint64, allocAmount uint64) ([]chain.AuthFactory, genesis.Genesis, error) {
	factories := make([]chain.AuthFactory, numOfFactories)
	customAllocs := make([]*genesis.CustomAllocation, numOfFactories)
	for i := range numOfFactories {
		pk, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return nil, nil, err
		}
		factory := auth.NewED25519Factory(pk)
		factories[i] = factory
		customAllocs[i] = &genesis.CustomAllocation{
			Address: factory.Address(),
			Balance: allocAmount,
		}
	}
	return factories, genesis.NewDefaultGenesis(customAllocs), nil
}

func createTestView(mp map[string][]byte) (merkledb.View, error) {
	db, err := merkledb.New(
		context.Background(),
		memdb.New(),
		merkledb.Config{
			BranchFactor: merkledb.BranchFactor16,
			Tracer:       trace.Noop,
		},
	)
	if err != nil {
		return nil, err
	}

	for key, value := range mp {
		if err := db.Put([]byte(key), value); err != nil {
			return nil, err
		}
	}

	return db, nil
}

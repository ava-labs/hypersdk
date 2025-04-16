// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
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
	ruleFactory := chaintest.RuleFactory()
	benchmarks := []struct {
		name                   string
		genesisGenerator       chaintest.GenesisGenerator[string]
		stateAccessDistributor chaintest.StateAccessDistributor[string]
		numTxsPerBlock         uint64
	}{
		{
			name:                   "empty",
			genesisGenerator:       noopGenesisF,
			stateAccessDistributor: chaintest.NoopDistributor[string]{},
		},
		{
			name:                   "parallel",
			genesisGenerator:       uniqueKeyGenesisGenerator,
			stateAccessDistributor: chaintest.NewParallelDistributor(actionConstructor{}, ruleFactory),
			numTxsPerBlock:         16,
		},
		{
			name:                   "serial",
			genesisGenerator:       singleKeyGenesisGenerator,
			stateAccessDistributor: chaintest.NewSerialDistributor(actionConstructor{}, ruleFactory),
			numTxsPerBlock:         16,
		},
		{
			name:                   "zipf",
			genesisGenerator:       uniqueKeyGenesisGenerator,
			stateAccessDistributor: chaintest.NewZipfDistributor(actionConstructor{}, ruleFactory),
			numTxsPerBlock:         16,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			benchmark := &chaintest.BlockBenchmark[string]{
				MetadataManager:        metadata.NewDefaultManager(),
				BalanceHandler:         balance.NewPrefixBalanceHandler([]byte{metadata.DefaultMinimumPrefix}),
				AuthEngines:            auth.DefaultEngines(),
				RuleFactory:            chaintest.RuleFactory(),
				GenesisF:               bm.genesisGenerator,
				StateAccessDistributor: bm.stateAccessDistributor,
				Config: chain.Config{
					TargetBuildDuration:       100 * time.Millisecond,
					TransactionExecutionCores: 4,
					StateFetchConcurrency:     4,
					TargetTxsSize:             1.5 * units.MiB,
				},
				NumBlocks:      1_000,
				NumTxsPerBlock: bm.numTxsPerBlock,
			}
			benchmark.Run(context.Background(), b)
		})
	}
}

func noopGenesisF(uint64) ([]chain.AuthFactory, []string, genesis.Genesis, error) {
	return nil, nil, genesis.NewDefaultGenesis(nil), nil
}

func uniqueKeyGenesisGenerator(numTxsPerBlock uint64) ([]chain.AuthFactory, []string, genesis.Genesis, error) {
	factories, genesis, err := chaintest.CreateGenesis(numTxsPerBlock, 1_000_000, chaintest.ED25519Factory)
	if err != nil {
		return nil, nil, nil, err
	}
	keys := make([]string, numTxsPerBlock)
	for i := range numTxsPerBlock {
		keys[i] = string(binary.BigEndian.AppendUint16(nil, uint16(i)))
	}

	return factories, keys, genesis, nil
}

func singleKeyGenesisGenerator(numTxsPerBlock uint64) ([]chain.AuthFactory, []string, genesis.Genesis, error) {
	factories, genesis, err := chaintest.CreateGenesis(numTxsPerBlock, 1_000_000, chaintest.ED25519Factory)
	if err != nil {
		return nil, nil, nil, err
	}

	keys := []string{string(binary.BigEndian.AppendUint16(nil, uint16(0)))}

	return factories, keys, genesis, nil
}

type actionConstructor struct{}

func (actionConstructor) Generate(k string, nonce uint64) chain.Action {
	return &chaintest.TestAction{
		Nonce:                        nonce,
		SpecifiedStateKeys:           []string{k},
		SpecifiedStateKeyPermissions: []state.Permissions{state.Write},
		WriteKeys:                    [][]byte{[]byte(k)},
		Start:                        -1,
		End:                          -1,
	}
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

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"encoding/binary"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
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
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	errMockVerifyExpiryReplayProtection = errors.New("mock validity window error")

	_ chaintest.BlockBenchmarkHelper = (*parallelTxBlockHelper)(nil)
	_ chaintest.BlockBenchmarkHelper = (*serialTxBlockHelper)(nil)
	_ chaintest.BlockBenchmarkHelper = (*zipfTxBlockHelper)(nil)
)

func TestProcessorExecute(t *testing.T) {
	testRules := genesis.NewDefaultRules()

	testMetadataManager := metadata.NewDefaultManager()
	feeKey := string(chain.FeeKey(testMetadataManager.FeePrefix()))
	heightKey := string(chain.HeightKey(testMetadataManager.HeightPrefix()))
	timestampKey := string(chain.TimestampKey(testMetadataManager.TimestampPrefix()))

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
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				p, err := ed25519.GeneratePrivateKey()
				r.NoError(err)

				tx, err := chain.NewTransaction(
					chain.Base{
						Timestamp: utils.UnixRMilli(
							testRules.GetMinEmptyBlockGap(),
							testRules.GetValidityWindow(),
						),
					},
					[]chain.Action{},
					&auth.ED25519{
						Signer: p.PublicKey(),
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
				chaintest.NewDummyTestAuthVM(),
				testMetadataManager,
				&mockBalanceHandler{},
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

func BenchmarExecuteEmptyBlocks(b *testing.B) {
	test := &chaintest.BlockBenchmark{
		MetadataManager: metadata.NewDefaultManager(),
		BalanceHandler:  &mockBalanceHandler{},
		AuthVM: &chaintest.TestAuthVM{
			GetAuthBatchVerifierF: func(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
				bv, ok := auth.Engines()[authTypeID]
				if !ok {
					return nil, false
				}
				return bv.GetBatchVerifier(cores, count), ok
			},
			Log: logging.NoLog{},
		},
		RuleFactory:          &genesis.ImmutableRuleFactory{Rules: genesis.NewDefaultRules()},
		BlockBenchmarkHelper: chaintest.NoopBlockBenchmarkHelper{},
		Config:               chain.NewDefaultConfig(),
		NumOfBlocks:          10_000,
		NumOfTxsPerBlock:     0,
	}
	test.Run(context.Background(), b)
}

type parallelTxBlockHelper struct {
	factories    []chain.AuthFactory
	databaseKeys []string
	values       [][]byte
	nonce        uint64
}

func (p *parallelTxBlockHelper) GenerateGenesis(numOfTxsPerBlock uint64) (genesis.Genesis, error) {
	factories, genesis, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, err
	}
	p.factories = factories

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

	p.databaseKeys = databaseKeys
	p.values = values

	return genesis, nil
}

func (p *parallelTxBlockHelper) GenerateTxList(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
	txs := make([]*chain.Transaction, numOfTxsPerBlock)
	for i := range numOfTxsPerBlock {
		action := &chaintest.TestAction{
			Nonce:                        p.nonce,
			SpecifiedStateKeys:           []string{p.databaseKeys[i]},
			SpecifiedStateKeyPermissions: []state.Permissions{state.Write},
			WriteKeys:                    [][]byte{[]byte(p.databaseKeys[i])},
			WriteValues:                  [][]byte{p.values[i]},
			Start:                        -1,
			End:                          -1,
		}

		p.nonce++

		tx, err := txGenerator([]chain.Action{action}, p.factories[i])
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}
	return txs, nil
}

func BenchmarkExecuteBlocksParallelTxs(b *testing.B) {
	test := &chaintest.BlockBenchmark{
		MetadataManager: metadata.NewDefaultManager(),
		BalanceHandler:  &mockBalanceHandler{},
		AuthVM: &chaintest.TestAuthVM{
			GetAuthBatchVerifierF: func(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
				bv, ok := auth.Engines()[authTypeID]
				if !ok {
					return nil, false
				}
				return bv.GetBatchVerifier(cores, count), ok
			},
			Log: logging.NoLog{},
		},
		RuleFactory:          &genesis.ImmutableRuleFactory{Rules: genesis.NewDefaultRules()},
		BlockBenchmarkHelper: &parallelTxBlockHelper{},
		Config:               chain.NewDefaultConfig(),
		NumOfBlocks:          1_000,
		NumOfTxsPerBlock:     16,
	}
	test.Run(context.Background(), b)
}

type serialTxBlockHelper struct {
	factories []chain.AuthFactory
	nonce     uint64
}

func (s *serialTxBlockHelper) GenerateGenesis(numOfTxsPerBlock uint64) (genesis.Genesis, error) {
	factories, genesis, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, err
	}
	s.factories = factories
	return genesis, nil
}

func (s *serialTxBlockHelper) GenerateTxList(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
	txs := make([]*chain.Transaction, numOfTxsPerBlock)

	k := string(binary.BigEndian.AppendUint16(nil, 1))
	v := binary.BigEndian.AppendUint64(nil, 1)

	for i := range numOfTxsPerBlock {
		action := &chaintest.TestAction{
			Nonce:                        s.nonce,
			SpecifiedStateKeys:           []string{k},
			SpecifiedStateKeyPermissions: []state.Permissions{state.Write},
			WriteKeys:                    [][]byte{v},
			Start:                        -1,
			End:                          -1,
		}

		s.nonce++

		tx, err := txGenerator([]chain.Action{action}, s.factories[i])
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}

	return txs, nil
}

func BenchmarkExecuteBlocksSerialTxs(b *testing.B) {
	test := &chaintest.BlockBenchmark{
		MetadataManager: metadata.NewDefaultManager(),
		BalanceHandler:  &mockBalanceHandler{},
		AuthVM: &chaintest.TestAuthVM{
			GetAuthBatchVerifierF: func(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
				bv, ok := auth.Engines()[authTypeID]
				if !ok {
					return nil, false
				}
				return bv.GetBatchVerifier(cores, count), ok
			},
			Log: logging.NoLog{},
		},
		RuleFactory:          &genesis.ImmutableRuleFactory{Rules: genesis.NewDefaultRules()},
		BlockBenchmarkHelper: &serialTxBlockHelper{},
		Config:               chain.NewDefaultConfig(),
		NumOfBlocks:          1_000,
		NumOfTxsPerBlock:     16,
	}
	test.Run(context.Background(), b)
}

type zipfTxBlockHelper struct {
	factories    []chain.AuthFactory
	zipfGen      *rand.Zipf
	nonce        uint64
	databaseKeys []string
	values       [][]byte
}

func (z *zipfTxBlockHelper) GenerateGenesis(numOfTxsPerBlock uint64) (genesis.Genesis, error) {
	factories, genesis, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, err
	}
	z.factories = factories

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

	z.databaseKeys = databaseKeys
	z.values = values

	zipfSeed := rand.New(rand.NewSource(0)) //nolint:gosec
	sZipf := 1.01
	vZipf := 2.7
	z.zipfGen = rand.NewZipf(zipfSeed, sZipf, vZipf, numOfTxsPerBlock-1)

	return genesis, nil
}

func (z *zipfTxBlockHelper) GenerateTxList(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
	txs := make([]*chain.Transaction, numOfTxsPerBlock)
	for i := range numOfTxsPerBlock {
		index := z.zipfGen.Uint64()
		action := &chaintest.TestAction{
			Nonce:                        z.nonce,
			SpecifiedStateKeys:           []string{z.databaseKeys[index]},
			SpecifiedStateKeyPermissions: []state.Permissions{state.Write},
			WriteKeys:                    [][]byte{[]byte(z.databaseKeys[index])},
			WriteValues:                  [][]byte{z.values[index]},
			Start:                        -1,
			End:                          -1,
		}

		z.nonce++

		tx, err := txGenerator([]chain.Action{action}, z.factories[i])
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}
	return txs, nil
}

func BenchmarkExecuteBlocksZipfTxs(b *testing.B) {
	test := &chaintest.BlockBenchmark{
		MetadataManager: metadata.NewDefaultManager(),
		BalanceHandler:  &mockBalanceHandler{},
		AuthVM: &chaintest.TestAuthVM{
			GetAuthBatchVerifierF: func(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
				bv, ok := auth.Engines()[authTypeID]
				if !ok {
					return nil, false
				}
				return bv.GetBatchVerifier(cores, count), ok
			},
			Log: logging.NoLog{},
		},
		RuleFactory:          &genesis.ImmutableRuleFactory{Rules: genesis.NewDefaultRules()},
		BlockBenchmarkHelper: &zipfTxBlockHelper{},
		Config:               chain.NewDefaultConfig(),
		NumOfBlocks:          1_000,
		NumOfTxsPerBlock:     16,
	}
	test.Run(context.Background(), b)
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

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state/metadata"
)

func BenchmarkMorpheusParallelExecution(b *testing.B) {
	r := require.New(b)

	numOfTxsPerBlock := uint64(5_000)

	factories, gen, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	r.NoError(err)

	rules := genesis.NewDefaultRules()
	// We maximize the window target units to avoid fund exhaustion from fee spikes.
	rules.WindowTargetUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	rules.MaxBlockUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	ruleFactory := &genesis.ImmutableRuleFactory{Rules: rules}

	test := &chaintest.BlockBenchmark{
		MetadataManager: metadata.NewDefaultManager(),
		BalanceHandler:  &storage.BalanceHandler{},
		Genesis:         gen,
		AuthVM: &chaintest.TestAuthVM{
			GetAuthBatchVerifierF: getAuthBatchVerifier,
			Log:                   logging.NoLog{},
		},
		Config: chain.Config{
			TargetBuildDuration:       100 * time.Millisecond,
			TransactionExecutionCores: 4,
			StateFetchConcurrency:     4,
			TargetTxsSize:             1.5 * units.MiB,
		},
		AuthVerificationCores: 8,
		RuleFactory:           ruleFactory,
		TxListGenerator:       NewParallelTxListGenerator(factories),
		NumOfBlocks:           10,
		NumOfTxsPerBlock:      numOfTxsPerBlock,
	}

	test.Run(context.Background(), b)
}

func BenchmarkMorpheusSerialExecution(b *testing.B) {
	r := require.New(b)

	numOfTxsPerBlock := uint64(5_000)

	factories, gen, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	r.NoError(err)

	rules := genesis.NewDefaultRules()
	// We maximize the window target units to avoid fund exhaustion from fee spikes.
	rules.WindowTargetUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	rules.MaxBlockUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	ruleFactory := &genesis.ImmutableRuleFactory{Rules: rules}

	test := &chaintest.BlockBenchmark{
		MetadataManager: metadata.NewDefaultManager(),
		BalanceHandler:  &storage.BalanceHandler{},
		Genesis:         gen,
		AuthVM: &chaintest.TestAuthVM{
			GetAuthBatchVerifierF: getAuthBatchVerifier,
			Log:                   logging.NoLog{},
		},
		Config: chain.Config{
			TargetBuildDuration:       100 * time.Millisecond,
			TransactionExecutionCores: 4,
			StateFetchConcurrency:     4,
			TargetTxsSize:             1.5 * units.MiB,
		},
		AuthVerificationCores: 8,
		RuleFactory:           ruleFactory,
		TxListGenerator:       NewSerialTxListGenerator(factories),
		NumOfBlocks:           10,
		NumOfTxsPerBlock:      numOfTxsPerBlock,
	}

	test.Run(context.Background(), b)
}

func BenchmarkMorpheusZipfExecution(b *testing.B) {
	r := require.New(b)

	numOfTxsPerBlock := uint64(5_000)

	factories, gen, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	r.NoError(err)

	rules := genesis.NewDefaultRules()
	// We maximize the window target units to avoid fund exhaustion from fee spikes.
	rules.WindowTargetUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	rules.MaxBlockUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	ruleFactory := &genesis.ImmutableRuleFactory{Rules: rules}

	test := &chaintest.BlockBenchmark{
		MetadataManager: metadata.NewDefaultManager(),
		BalanceHandler:  &storage.BalanceHandler{},
		Genesis:         gen,
		AuthVM: &chaintest.TestAuthVM{
			GetAuthBatchVerifierF: getAuthBatchVerifier,
			Log:                   logging.NoLog{},
		},
		Config: chain.Config{
			TargetBuildDuration:       100 * time.Millisecond,
			TransactionExecutionCores: 4,
			StateFetchConcurrency:     4,
			TargetTxsSize:             1.5 * units.MiB,
		},
		AuthVerificationCores: 8,
		RuleFactory:           ruleFactory,
		TxListGenerator:       NewZipfTxListGenerator(factories),
		NumOfBlocks:           10,
		NumOfTxsPerBlock:      numOfTxsPerBlock,
	}

	test.Run(context.Background(), b)
}

func getAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	bv, ok := auth.Engines()[authTypeID]
	if !ok {
		return nil, false
	}
	return bv.GetBatchVerifier(cores, count), true
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

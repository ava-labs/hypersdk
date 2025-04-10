// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state/metadata"
)

func BenchmarkMorpheusBlocks(b *testing.B) {
	benchmarks := []struct {
		name                   string
		genesisGenerator       chaintest.GenesisGenerator[codec.Address]
		actionConstructor      chaintest.ActionConstructor[codec.Address]
		stateAccessDistributor chaintest.StateAccessDistributor[codec.Address]
	}{
		{
			name:                   "parallel transfers",
			genesisGenerator:       uniqueAddressGenesisF,
			actionConstructor:      actionGenerator,
			stateAccessDistributor: chaintest.ParallelDistribution[codec.Address],
		},
		{
			name:                   "serial transfers",
			genesisGenerator:       singleAddressGenesisF,
			actionConstructor:      actionGenerator,
			stateAccessDistributor: chaintest.SerialDistribution[codec.Address],
		},
		{
			name:                   "zipf transfers",
			genesisGenerator:       uniqueAddressGenesisF,
			actionConstructor:      actionGenerator,
			stateAccessDistributor: chaintest.ZipfDistribution[codec.Address],
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			benchmark := &chaintest.BlockBenchmark[codec.Address]{
				MetadataManager:        metadata.NewDefaultManager(),
				BalanceHandler:         &storage.BalanceHandler{},
				RuleFactory:            chaintest.RuleFactory(),
				AuthEngines:            auth.DefaultEngines(),
				GenesisF:               bm.genesisGenerator,
				ActionConstructor:      bm.actionConstructor,
				StateAccessDistributor: bm.stateAccessDistributor,
				Config: chain.Config{
					TargetBuildDuration:       100 * time.Millisecond,
					TransactionExecutionCores: 4,
					StateFetchConcurrency:     4,
					TargetTxsSize:             1.5 * units.MiB,
				},
				AuthVerificationCores: 8,
				NumBlocks:             10,
				NumTxsPerBlock:        5_000,
			}

			benchmark.Run(context.Background(), b)
		})
	}
}

func actionGenerator(k codec.Address, nonce uint64) chain.Action {
	return &actions.Transfer{
		To:    k,
		Value: 1,
		Memo:  binary.BigEndian.AppendUint64(nil, nonce),
	}
}

func uniqueAddressGenesisF(numTxsPerBlock uint64) ([]chain.AuthFactory, []codec.Address, genesis.Genesis, error) {
	factories, gen, err := chaintest.CreateGenesis(numTxsPerBlock, 1_000_000, chaintest.ED25519Factory)
	if err != nil {
		return nil, nil, nil, err
	}

	keys := make([]codec.Address, len(factories))
	for i, factory := range factories {
		keys[i] = factory.Address()
	}

	return factories, keys, gen, err
}

func singleAddressGenesisF(numTxsPerBlock uint64) ([]chain.AuthFactory, []codec.Address, genesis.Genesis, error) {
	factories, gen, err := chaintest.CreateGenesis(numTxsPerBlock, 1_000_000, chaintest.ED25519Factory)
	if err != nil {
		return nil, nil, nil, err
	}

	return factories, []codec.Address{codec.EmptyAddress}, gen, err
}

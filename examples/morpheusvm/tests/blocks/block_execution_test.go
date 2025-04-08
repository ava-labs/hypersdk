// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"context"
	"encoding/binary"
	"math/rand"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state/metadata"
)

func BenchmarkMorpheusBlocks(b *testing.B) {
	rules := genesis.NewDefaultRules()
	// we maximize the window target units and max block units to avoid fund exhaustion from fee spikes.
	rules.WindowTargetUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	rules.MaxBlockUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	ruleFactory := &genesis.ImmutableRuleFactory{Rules: rules}

	benchmarks := []struct {
		name                 string
		blockBenchmarkHelper chaintest.BlockBenchmarkHelper
	}{
		{
			name:                 "parallel transfers",
			blockBenchmarkHelper: parallelTxsBlockBenchmarkHelper,
		},
		{
			name:                 "serial transfers",
			blockBenchmarkHelper: serialTxsBlockBenchmarkHelper,
		},
		{
			name:                 "zipf transfers",
			blockBenchmarkHelper: zipfTxsBlockBenchmarkHelper,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			benchmark := &chaintest.BlockBenchmark{
				MetadataManager:      metadata.NewDefaultManager(),
				BalanceHandler:       &storage.BalanceHandler{},
				RuleFactory:          ruleFactory,
				AuthEngines:          auth.DefaultEngines(),
				BlockBenchmarkHelper: bm.blockBenchmarkHelper,
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

func parallelTxsBlockBenchmarkHelper(numTxsPerBlock uint64) (genesis.Genesis, chaintest.TxListGenerator, error) {
	factories, gen, err := createGenesis(numTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, nil, err
	}

	nonce := uint64(0)

	txListGenerator := func(ruleFactory chain.RuleFactory, unitPrices fees.Dimensions, timestamp int64) ([]*chain.Transaction, error) {
		txs := make([]*chain.Transaction, numTxsPerBlock)
		for i := 0; i < int(numTxsPerBlock); i++ {
			action := &actions.Transfer{
				To:    factories[i].Address(),
				Value: 1,
				Memo:  binary.BigEndian.AppendUint64(nil, nonce),
			}

			nonce++

			tx, err := chain.GenerateTransaction(
				ruleFactory,
				unitPrices,
				timestamp,
				[]chain.Action{action},
				factories[i],
			)
			if err != nil {
				return nil, err
			}

			txs[i] = tx
		}

		return txs, nil
	}
	return gen, txListGenerator, nil
}

func serialTxsBlockBenchmarkHelper(numTxsPerBlock uint64) (genesis.Genesis, chaintest.TxListGenerator, error) {
	factories, gen, err := createGenesis(numTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, nil, err
	}

	nonce := uint64(0)

	txListGenerator := func(ruleFactory chain.RuleFactory, unitPrices fees.Dimensions, timestamp int64) ([]*chain.Transaction, error) {
		txs := make([]*chain.Transaction, numTxsPerBlock)
		for i := 0; i < int(numTxsPerBlock); i++ {
			action := &actions.Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
				Memo:  binary.BigEndian.AppendUint64(nil, nonce),
			}

			nonce++

			tx, err := chain.GenerateTransaction(
				ruleFactory,
				unitPrices,
				timestamp,
				[]chain.Action{action},
				factories[i],
			)
			if err != nil {
				return nil, err
			}

			txs[i] = tx
		}

		return txs, nil
	}
	return gen, txListGenerator, nil
}

func zipfTxsBlockBenchmarkHelper(numTxsPerBlock uint64) (genesis.Genesis, chaintest.TxListGenerator, error) {
	factories, gen, err := createGenesis(numTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, nil, err
	}

	nonce := uint64(0)

	zipfSeed := rand.New(rand.NewSource(0)) //nolint:gosec
	sZipf := 1.01
	vZipf := 2.7
	zipfGen := rand.NewZipf(zipfSeed, sZipf, vZipf, numTxsPerBlock-1)

	txListGenerator := func(ruleFactory chain.RuleFactory, unitPrices fees.Dimensions, timestamp int64) ([]*chain.Transaction, error) {
		txs := make([]*chain.Transaction, numTxsPerBlock)
		for i := 0; i < int(numTxsPerBlock); i++ {
			action := &actions.Transfer{
				To:    factories[zipfGen.Uint64()].Address(),
				Value: 1,
				Memo:  binary.BigEndian.AppendUint64(nil, nonce),
			}

			nonce++

			tx, err := chain.GenerateTransaction(
				ruleFactory,
				unitPrices,
				timestamp,
				[]chain.Action{action},
				factories[i],
			)
			if err != nil {
				return nil, err
			}

			txs[i] = tx
		}

		return txs, nil
	}
	return gen, txListGenerator, nil
}

func createGenesis(numAccounts uint64, allocAmount uint64) ([]chain.AuthFactory, genesis.Genesis, error) {
	factories := make([]chain.AuthFactory, numAccounts)
	customAllocs := make([]*genesis.CustomAllocation, numAccounts)
	for i := range numAccounts {
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

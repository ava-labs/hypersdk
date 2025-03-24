// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/consts"
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
			name:                 "transfer txs with disjoint recipients",
			blockBenchmarkHelper: parallelTxsBlockBenchmarkHelper,
		},
		{
			name:                 "transfer txs that all send to the same recipient",
			blockBenchmarkHelper: serialTxsBlockBenchmarkHelper,
		},
		{
			name:                 "transfer txs whose recipient is sampled from the zipf distribution",
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
				NumOfBlocks:           10,
				NumOfTxsPerBlock:      5_000,
			}

			benchmark.Run(context.Background(), b)
		})
	}
}

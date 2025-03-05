// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
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

func BenchmarkMorpheusParallelExecution(b *testing.B) {
	benchmarkMorpheusBlocks(b, &ParallelTxListGenerator{})
}

func BenchmarkMorpheusSerialExecution(b *testing.B) {
	benchmarkMorpheusBlocks(b, &SerialTxListGenerator{})
}

func BenchmarkMorpheusZipfExecution(b *testing.B) {
	benchmarkMorpheusBlocks(b, &ZipfTxListGenerator{})
}

func benchmarkMorpheusBlocks(b *testing.B, txListGenerator chaintest.TxListGenerator) {
	rules := genesis.NewDefaultRules()
	// We maximize the window target units to avoid fund exhaustion from fee spikes.
	rules.WindowTargetUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	rules.MaxBlockUnits = fees.Dimensions{20_000_000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	ruleFactory := &genesis.ImmutableRuleFactory{Rules: rules}

	test := &chaintest.BlockBenchmark{
		MetadataManager: metadata.NewDefaultManager(),
		BalanceHandler:  &storage.BalanceHandler{},
		AuthVM: &chaintest.TestAuthVM{
			GetAuthBatchVerifierF: func(authTypeID uint8, cores, count int) (chain.AuthBatchVerifier, bool) {
				bv, ok := auth.Engines()[authTypeID]
				if !ok {
					return nil, false
				}
				return bv.GetBatchVerifier(cores, count), true
			},
			Log: logging.NoLog{},
		},
		Config: chain.Config{
			TargetBuildDuration:       100 * time.Millisecond,
			TransactionExecutionCores: 4,
			StateFetchConcurrency:     4,
			TargetTxsSize:             1.5 * units.MiB,
		},
		AuthVerificationCores: 8,
		RuleFactory:           ruleFactory,
		TxListGenerator:       txListGenerator,
		NumOfBlocks:           10,
		NumOfTxsPerBlock:      5_000,
	}

	test.Run(context.Background(), b)
}

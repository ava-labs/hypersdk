// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"
)

func TestStateAccessDistributorGenerate(t *testing.T) {
	ruleFactory := RuleFactory()
	tests := []struct {
		name                   string
		stateAccessDistributor StateAccessDistributor[uint64]
		numTxs                 uint64
		keys                   []uint64
		factories              []chain.AuthFactory
		expectedErr            error
	}{
		{
			name:                   "parallel - numTxs != len(keys)",
			stateAccessDistributor: NewParallelDistributor[uint64](nil, ruleFactory),
			keys:                   []uint64{1},
			expectedErr:            ErrMismatchedKeysAndFactoriesLen,
		},
		{
			name:                   "parallel - numTxs != len(factories)",
			stateAccessDistributor: NewParallelDistributor[uint64](nil, ruleFactory),
			factories:              make([]chain.AuthFactory, 1),
			expectedErr:            ErrMismatchedKeysAndFactoriesLen,
		},
		{
			name:                   "serial - len(keys) != 1",
			stateAccessDistributor: NewSerialDistributor[uint64](nil, ruleFactory),
			expectedErr:            ErrSingleKeyLengthOnly,
		},
		{
			name:                   "zipf - numTxs != len(keys)",
			stateAccessDistributor: NewZipfDistributor[uint64](nil, ruleFactory),
			keys:                   []uint64{1},
			expectedErr:            ErrMismatchedKeysAndFactoriesLen,
		},
		{
			name:                   "zipf - numTxs != len(factories)",
			stateAccessDistributor: NewZipfDistributor[uint64](nil, ruleFactory),
			factories:              make([]chain.AuthFactory, 1),
			expectedErr:            ErrMismatchedKeysAndFactoriesLen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			_, err := tt.stateAccessDistributor.Generate(
				tt.numTxs,
				tt.factories,
				tt.keys,
				fees.Dimensions{},
				time.Now().UnixMilli(),
			)
			r.ErrorIs(err, tt.expectedErr)
		})
	}
}

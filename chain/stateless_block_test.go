// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/state"
	"github.com/stretchr/testify/require"
)

func TestSerializeBlock(t *testing.T) {
	type test struct {
		name        string
		createBlock func(r *require.Assertions) (*chain.StatelessBlock, chain.Parser)
	}

	testParser := chaintest.NewTestParser()
	for _, test := range []test{
		{
			name: "empty block",
			createBlock: func(r *require.Assertions) (*chain.StatelessBlock, chain.Parser) {
				block, err := chain.NewStatelessBlock(
					ids.GenerateTestID(),
					123,
					1,
					nil, // Note: canoto serializes a nil slice identically to an empty slice, but the equality check will fail if we use an empty slice instead of nil here
					ids.GenerateTestID(),
					nil,
				)
				r.NoError(err)
				return block, testParser
			},
		},
		{
			name: "non-empty block",
			createBlock: func(r *require.Assertions) (*chain.StatelessBlock, chain.Parser) {
				tx, err := chain.NewTransaction(
					&chain.Base{
						Timestamp: 1_000,
						ChainID:   ids.GenerateTestID(),
						MaxFee:    1,
					},
					[]chain.Action{
						&chaintest.TestAction{
							NumComputeUnits:    1,
							SpecifiedStateKeys: make(state.Keys),
							ReadKeys:           [][]byte{{1, 2, 3}},
							WriteKeys:          [][]byte{{4, 5, 6}},
							WriteValues:        [][]byte{{7, 8, 9}},
						},
					},
					&chaintest.TestAuth{},
				)
				r.NoError(err)
				block, err := chain.NewStatelessBlock(
					ids.GenerateTestID(),
					123,
					1,
					[]*chain.Transaction{tx},
					ids.GenerateTestID(),
					nil,
				)
				r.NoError(err)
				return block, testParser
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)

			block, parser := test.createBlock(r)
			unmarshalledBlock, err := chain.UnmarshalBlock(block.GetBytes(), parser)
			r.NoError(err)
			r.EqualValues(block, unmarshalledBlock)
		})
	}
}

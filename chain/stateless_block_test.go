// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
)

func TestBlockSerialization(t *testing.T) {
	type test struct {
		name        string
		createBlock func(r *require.Assertions) (expectedBlock *chain.StatelessBlock, blockBytes []byte)
	}

	testParser := chaintest.NewTestParser()
	for _, test := range []test{
		{
			name: "empty block",
			createBlock: func(r *require.Assertions) (*chain.StatelessBlock, []byte) {
				blockArgs := createBlockArgs(r, 0)
				block, err := chain.NewStatelessBlock(
					blockArgs.parentID,
					blockArgs.timestamp,
					blockArgs.height,
					blockArgs.txs,
					blockArgs.stateRoot,
					nil,
				)
				r.NoError(err)
				return block, block.GetBytes()
			},
		},
		{
			name: "single tx block",
			createBlock: func(r *require.Assertions) (*chain.StatelessBlock, []byte) {
				blockArgs := createBlockArgs(r, 1)
				block, err := chain.NewStatelessBlock(
					blockArgs.parentID,
					blockArgs.timestamp,
					blockArgs.height,
					blockArgs.txs,
					blockArgs.stateRoot,
					nil,
				)
				r.NoError(err)
				return block, block.GetBytes()
			},
		},
		{
			name: "non-empty block context",
			createBlock: func(r *require.Assertions) (*chain.StatelessBlock, []byte) {
				blockArgs := createBlockArgs(r, 1)
				block, err := chain.NewStatelessBlock(
					blockArgs.parentID,
					blockArgs.timestamp,
					blockArgs.height,
					blockArgs.txs,
					blockArgs.stateRoot,
					&block.Context{
						PChainHeight: 64,
					},
				)
				r.NoError(err)
				return block, block.GetBytes()
			},
		},
		{
			name: "multi tx block",
			createBlock: func(r *require.Assertions) (*chain.StatelessBlock, []byte) {
				blockArgs := createBlockArgs(r, 3)

				block, err := chain.NewStatelessBlock(
					blockArgs.parentID,
					blockArgs.timestamp,
					blockArgs.height,
					blockArgs.txs,
					blockArgs.stateRoot,
					nil,
				)
				r.NoError(err)
				return block, block.GetBytes()
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)

			expectedBlock, blockBytes := test.createBlock(r)
			unmarshalledBlock, err := chain.UnmarshalBlock(blockBytes, testParser)
			r.NoError(err)
			r.EqualValues(expectedBlock, unmarshalledBlock)
		})
	}
}

func TestParseHardcodedBlock(t *testing.T) {
	r := require.New(t)

	// Hardcoded bytes of the block to verify there are no unexpected serialization changes
	blockHex := "0a20e902a9a86640bfdb1cd0e36c0cc982b83e5765fad5f6bbe6abdcce7b5ae7d7c7117b000000000000001901000000000000002a7f0a2508d00f12203d0ad12b8ee8928edf248ca91ca55600fb383f07c32bff1d6dec472b25cf59a71207000801480150011a4d00080112210102030000000000000000000000000000000000000000000000000000000000001a21010203000000000000000000000000000000000000000000000000000000000000280130012a88010a2e08d00f12203d0ad12b8ee8928edf248ca91ca55600fb383f07c32bff1d6dec472b25cf59a71901000000000000001207000801480150011a4d00080112210102030000000000000000000000000000000000000000000000000000000000001a21010203000000000000000000000000000000000000000000000000000000000000280130012a88010a2e08d00f12203d0ad12b8ee8928edf248ca91ca55600fb383f07c32bff1d6dec472b25cf59a71902000000000000001207000801480150011a4d00080112210102030000000000000000000000000000000000000000000000000000000000001a210102030000000000000000000000000000000000000000000000000000000000002801300132204a177205df5c29929d06db9d941f83d5ea985de302015e99252d16469a6610db"
	hardcodedBlockBytes, err := hex.DecodeString(blockHex)
	r.NoError(err)

	txs := make([]*chain.Transaction, 0, 3)
	chainID := ids.Empty.Prefix(1)
	for i := 0; i < 3; i++ {
		tx, err := chain.NewTransaction(
			chain.Base{
				Timestamp: 1_000,
				ChainID:   chainID,
				MaxFee:    uint64(i),
			},
			[]chain.Action{
				chaintest.NewDummyTestAction(),
			},
			chaintest.NewDummyTestAuth(),
		)
		r.NoError(err)
		txs = append(txs, tx)
	}

	block, err := chain.NewStatelessBlock(
		ids.Empty.Prefix(2),
		123,
		1,
		txs,
		ids.Empty.Prefix(3),
		nil,
	)
	r.NoError(err)
	blockBytes := block.GetBytes()

	r.Equal(hardcodedBlockBytes, blockBytes, "expected %x, actual %x", hardcodedBlockBytes, blockBytes)
}

func TestBlockWithNilTransaction(t *testing.T) {
	r := require.New(t)

	blockArgs := createBlockArgs(r, 1)
	parser := chaintest.NewTestParser()

	block, err := chain.NewStatelessBlock(
		blockArgs.parentID,
		blockArgs.timestamp,
		blockArgs.height,
		[]*chain.Transaction{blockArgs.txs[0], nil},
		blockArgs.stateRoot,
		nil,
	)
	r.NoError(err)

	blockBytes := block.GetBytes()

	_, err = chain.UnmarshalBlock(blockBytes, parser)
	r.ErrorIs(err, chain.ErrNilTxInBlock)
}

// go test -benchmem -run=^$ -bench ^BenchmarkUnmarshalBlock$ github.com/ava-labs/hypersdk/chain -timeout=15s
//
// goos: darwin
// goarch: arm64
// pkg: github.com/ava-labs/hypersdk/chain
// BenchmarkUnmarshalBlock/UnmarshalBlock-0-txs-10                   977838              1111 ns/op             176 B/op          1 allocs/op
// BenchmarkUnmarshalBlock/UnmarshalBlock-1-txs-10                   658054              1817 ns/op             904 B/op         10 allocs/op
// BenchmarkUnmarshalBlock/UnmarshalBlock-10-txs-10                  150804              7924 ns/op            7456 B/op         82 allocs/op
// BenchmarkUnmarshalBlock/UnmarshalBlock-1000-txs-10                  1689            684934 ns/op          728377 B/op       8002 allocs/op
// BenchmarkUnmarshalBlock/UnmarshalBlock-10000-txs-10                  172           6897857 ns/op         7282186 B/op      80002 allocs/op
// BenchmarkUnmarshalBlock/UnmarshalBlock-100000-txs-10                  19          66020735 ns/op        72803047 B/op     800002 allocs/op
func BenchmarkUnmarshalBlock(b *testing.B) {
	for _, numTxs := range []int{0, 1, 10, 1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("UnmarshalBlock-%d-txs", numTxs), func(b *testing.B) {
			r := require.New(b)

			blockArgs := createBlockArgs(r, numTxs)
			block, err := chain.NewStatelessBlock(
				blockArgs.parentID,
				blockArgs.timestamp,
				blockArgs.height,
				blockArgs.txs,
				blockArgs.stateRoot,
				nil,
			)
			r.NoError(err)

			blockBytes := block.GetBytes()
			parser := chaintest.NewTestParser()

			parsedBlk, err := chain.UnmarshalBlock(blockBytes, parser)
			r.NoError(err)
			r.EqualValues(block, parsedBlk)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				parsedBlk, err := chain.UnmarshalBlock(blockBytes, parser)
				r.NoError(err)
				r.NotNil(parsedBlk)
			}
		})
	}
}

// BenchmarkNewStatelessBlock benchmarks the construction of a stateless block instance
// rather than a marshal function, since we create a block either via the constructor
// or by unmarshalling. In both cases, we populate an internal bytes field, so that we
// never need to re-marshal the block. Therefore, we benchmark the constructor rather
// than a marshal function directly.
//
// go test -benchmem -run=^$ -bench ^BenchmarkNewStatelessBlock$ github.com/ava-labs/hypersdk/chain -timeout=15s
//
// goos: darwin
// goarch: arm64
// pkg: github.com/ava-labs/hypersdk/chain
// BenchmarkNewStatelessBlock/NewStatelessBlock-0-txs-12         	 1377030	       885.0 ns/op	     272 B/op	       2 allocs/op
// BenchmarkNewStatelessBlock/NewStatelessBlock-1-txs-12         	 1219345	       985.3 ns/op	     512 B/op	       2 allocs/op
// BenchmarkNewStatelessBlock/NewStatelessBlock-10-txs-12        	  504799	      2286 ns/op	    2496 B/op	       2 allocs/op
// BenchmarkNewStatelessBlock/NewStatelessBlock-1000-txs-12      	    8767	    149965 ns/op	  229569 B/op	       2 allocs/op
// BenchmarkNewStatelessBlock/NewStatelessBlock-10000-txs-12     	     973	   1237417 ns/op	 2228419 B/op	       2 allocs/op
// BenchmarkNewStatelessBlock/NewStatelessBlock-100000-txs-12    	      86	  13077714 ns/op	22200515 B/op	       2 allocs/op
func BenchmarkNewStatelessBlock(b *testing.B) {
	for _, numTxs := range []int{0, 1, 10, 1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("NewStatelessBlock-%d-txs", numTxs), func(b *testing.B) {
			r := require.New(b)

			blockArgs := createBlockArgs(r, numTxs)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				block, err := chain.NewStatelessBlock(
					blockArgs.parentID,
					blockArgs.timestamp,
					blockArgs.height,
					blockArgs.txs,
					blockArgs.stateRoot,
					nil,
				)

				r.NoError(err)
				r.NotNil(block)
			}
		})
	}
}

// blockArgs provides the arguments to construct a block with the
// block context omitted.
type blockArgs struct {
	parentID  ids.ID
	height    uint64
	timestamp int64
	txs       []*chain.Transaction
	stateRoot ids.ID
}

// createBlockArgs creates an instance of the arguments to construct a block with the
// specified number of transactions
func createBlockArgs(r *require.Assertions, numTxs int) *blockArgs {
	chainID := ids.Empty.Prefix(0)
	parentID := ids.Empty.Prefix(1)
	stateRoot := ids.Empty.Prefix(2)
	// Generate transactions
	// Note: canoto serializes a nil slice identically to an empty slice, but the
	// equality check will fail if we use an empty slice instead of nil here.
	var txs []*chain.Transaction
	if numTxs > 0 {
		txs = make([]*chain.Transaction, 0, numTxs)
		for i := 0; i < numTxs; i++ {
			tx, err := chain.NewTransaction(
				chain.Base{
					Timestamp: 1_000,
					ChainID:   chainID,
					MaxFee:    uint64(i),
				},
				[]chain.Action{
					chaintest.NewDummyTestAction(),
				},
				chaintest.NewDummyTestAuth(),
			)
			r.NoError(err)
			txs = append(txs, tx)
		}
	}

	blockArgs := &blockArgs{
		parentID:  parentID,
		height:    123,
		timestamp: 1,
		txs:       txs,
		stateRoot: stateRoot,
	}
	return blockArgs
}

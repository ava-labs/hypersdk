// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/dsmr/dsmrtest"
)

func BenchmarkChunkInit(b *testing.B) {
	chunk := newChunk[*dsmrtest.Tx](
		UnsignedChunk[*dsmrtest.Tx]{
			Producer:    ids.GenerateTestNodeID(),
			Beneficiary: codec.Address{123},
			Expiry:      123,
			Txs: []*dsmrtest.Tx{
				{
					ID:      ids.GenerateTestID(),
					Expiry:  456,
					Sponsor: codec.Address{4, 5, 6},
				},
			},
		},
		[48]byte{},
		[96]byte{},
	)

	for range b.N {
		chunk.init()
	}
}

func BenchmarkParseChunk(b *testing.B) {
	chunk := newChunk[*dsmrtest.Tx](
		UnsignedChunk[*dsmrtest.Tx]{
			Producer:    ids.GenerateTestNodeID(),
			Beneficiary: codec.Address{123},
			Expiry:      123,
			Txs: []*dsmrtest.Tx{
				{
					ID:      ids.GenerateTestID(),
					Expiry:  456,
					Sponsor: codec.Address{4, 5, 6},
				},
			},
		},
		[48]byte{},
		[96]byte{},
	)

	for range b.N {
		_, _ = ParseChunk[*dsmrtest.Tx](chunk.bytes)
	}
}

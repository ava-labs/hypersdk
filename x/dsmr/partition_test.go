// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/x/dsmr/dsmrtest"
)

// createTestPartition creates a precalculated partition with the provided weights
// and generates sorted nodeIDs to match the original ordering of weights
func createTestPartition(weights []uint64) *Partition[dsmrtest.Tx] {
	nodeIDs := make([]ids.NodeID, 0, len(weights))
	for i := 0; i < len(weights); i++ {
		nodeIDs = append(nodeIDs, ids.GenerateTestNodeID())
	}
	utils.Sort(nodeIDs)

	validators := make([]Validator, len(weights))
	for i, weight := range weights {
		validators[i] = Validator{
			Weight: weight,
			NodeID: nodeIDs[i],
		}
	}
	return NewPartition[dsmrtest.Tx](validators)
}

func createTestPartitionTx(weight uint64) dsmrtest.Tx {
	sponsorAddrID := ids.GenerateTestID()
	binary.BigEndian.PutUint64(
		sponsorAddrID[len(sponsorAddrID)-consts.Uint64Len:],
		weight,
	)

	sponsorAddr := codec.CreateAddress(0, sponsorAddrID)
	return dsmrtest.Tx{
		ID:      ids.GenerateTestID(),
		Expiry:  100,
		Sponsor: sponsorAddr,
	}
}

func TestAssignTx(t *testing.T) {
	for _, test := range []struct {
		name          string
		weights       []uint64
		txWeight      uint64
		expectedIndex int
	}{
		{
			name: "zero weight",
			weights: []uint64{
				100,
				100,
			},
			txWeight:      0,
			expectedIndex: 0,
		},
		{
			name: "total weight",
			weights: []uint64{
				100,
				100,
			},
			txWeight:      200,
			expectedIndex: 0,
		},
		{
			name: "middle of first node",
			weights: []uint64{
				100,
				100,
			},
			txWeight:      50,
			expectedIndex: 0,
		},
		{
			name: "middle of last node",
			weights: []uint64{
				100,
				100,
			},
			txWeight:      150,
			expectedIndex: 1,
		},
		{
			name: "middle of interior node",
			weights: []uint64{
				100,
				100,
				100,
			},
			txWeight:      150,
			expectedIndex: 1,
		},
		{
			name: "upper boundary of interior node",
			weights: []uint64{
				100,
				100,
				100,
			},
			txWeight:      200,
			expectedIndex: 2,
		},
		{
			name: "lower boundary of interior node",
			weights: []uint64{
				100,
				100,
				100,
			},
			txWeight:      100,
			expectedIndex: 1,
		},
		{
			name: "upper boundary of first node",
			weights: []uint64{
				100,
				100,
				100,
			},
			txWeight:      100,
			expectedIndex: 1,
		},
		{
			name: "lower boundary of last node",
			weights: []uint64{
				100,
				100,
				100,
			},
			txWeight:      200,
			expectedIndex: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)
			partition := createTestPartition(test.weights)

			tx := createTestPartitionTx(test.txWeight)
			nodeID, ok := partition.AssignTx(tx)
			r.True(ok)
			foundNodeIDIndex := -1
			for i, vdr := range partition.validators {
				if vdr.nodeID == nodeID {
					foundNodeIDIndex = i
					break
				}
			}
			r.Equal(test.expectedIndex, foundNodeIDIndex)
			r.Equal(partition.validators[test.expectedIndex].nodeID, nodeID)
		})
	}
}

// Running tool: /usr/local/go/bin/go test -benchmem -run=^$ -bench ^BenchmarkAssignTx$ github.com/ava-labs/hypersdk/x/dsmr
//
// goos: darwin
// goarch: arm64
// pkg: github.com/ava-labs/hypersdk/x/dsmr
// BenchmarkAssignTx/Vdrs-10-Txs-10-12         	  332629	      3400 ns/op	    1792 B/op	       1 allocs/op
// BenchmarkAssignTx/Vdrs-10-Txs-100-12        	   36576	     32696 ns/op	   14360 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-10-Txs-1000-12       	    3703	    325971 ns/op	  229401 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-10-Txs-10000-12      	     356	   3234778 ns/op	 1777688 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-100-Txs-10-12        	  324730	      3539 ns/op	    1812 B/op	       1 allocs/op
// BenchmarkAssignTx/Vdrs-100-Txs-100-12       	   34230	     34997 ns/op	   14378 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-100-Txs-1000-12      	    3499	    341810 ns/op	  229400 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-100-Txs-10000-12     	     355	   3340795 ns/op	 1777688 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-1000-Txs-10-12       	  314222	      3685 ns/op	    1812 B/op	       1 allocs/op
// BenchmarkAssignTx/Vdrs-1000-Txs-100-12      	   33060	     36677 ns/op	   15559 B/op	       6 allocs/op
// BenchmarkAssignTx/Vdrs-1000-Txs-1000-12     	    3241	    368321 ns/op	  229408 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-1000-Txs-10000-12    	     327	   3657135 ns/op	 1777688 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-10000-Txs-10-12      	  295610	      3920 ns/op	    1812 B/op	       1 allocs/op
// BenchmarkAssignTx/Vdrs-10000-Txs-100-12     	   30014	     39876 ns/op	   15843 B/op	       6 allocs/op
// BenchmarkAssignTx/Vdrs-10000-Txs-1000-12    	    2899	    416381 ns/op	  229482 B/op	       5 allocs/op
// BenchmarkAssignTx/Vdrs-10000-Txs-10000-12   	     286	   4145923 ns/op	 1777898 B/op	       7 allocs/op
func BenchmarkAssignTx(b *testing.B) {
	vdrSetSizes := []int{10, 100, 1000, 10_000}
	txBatchSizes := []int{10, 100, 1000, 10_000}
	for _, numVdrs := range vdrSetSizes {
		// Create validator set with equal weights
		vdrWeights := make([]uint64, numVdrs)
		for i := 0; i < numVdrs; i++ {
			vdrWeights[i] = 100
		}
		partition := createTestPartition(vdrWeights)

		// Create txs once for largest batch size
		testTxs := make([]dsmrtest.Tx, txBatchSizes[len(txBatchSizes)-1])
		for i := range testTxs {
			testTxs[i] = dsmrtest.Tx{
				ID:      ids.GenerateTestID(),
				Expiry:  100,
				Sponsor: codec.CreateAddress(0, ids.GenerateTestID()),
			}
		}

		for _, numTxs := range txBatchSizes {
			b.Run("Vdrs-"+strconv.Itoa(numVdrs)+"-Txs-"+strconv.Itoa(numTxs), func(b *testing.B) {
				r := require.New(b)
				for n := 0; n < b.N; n++ {
					assignments := make(map[ids.NodeID]dsmrtest.Tx, numTxs)
					for _, tx := range testTxs[:numTxs] {
						nodeID, ok := partition.AssignTx(tx)
						r.True(ok)
						assignments[nodeID] = tx
					}
				}
			})
		}
	}
}

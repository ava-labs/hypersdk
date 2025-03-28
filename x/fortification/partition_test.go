// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fortification

import (
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

// createTestPartition creates a precalculated partition with the provided weights
// and generates sorted nodeIDs to match the original ordering of weights
func createTestPartition(weights []uint64) *Partition {
	nodeIDs := make([]ids.NodeID, 0, len(weights))
	for i := 0; i < len(weights); i++ {
		nodeIDs = append(nodeIDs, ids.GenerateTestNodeID())
	}
	utils.Sort(nodeIDs)

	validators := make([]WeightedValidator, len(weights))
	for i, weight := range weights {
		validators[i] = WeightedValidator{
			Weight: weight,
			NodeID: nodeIDs[i],
		}
	}
	return NewPartition(validators)
}

type testPartitionTx codec.Address

func (t testPartitionTx) GetSponsor() codec.Address {
	return codec.Address(t)
}

func createTestPartitionTx(weight uint64) testPartitionTx {
	sponsorAddrID := ids.GenerateTestID()
	binary.BigEndian.PutUint64(
		sponsorAddrID[len(sponsorAddrID)-consts.Uint64Len:],
		weight,
	)

	sponsorAddr := codec.CreateAddress(0, sponsorAddrID)
	return testPartitionTx(sponsorAddr)
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
			nodeID, ok := AssignTx(partition, tx)
			r.True(ok)
			foundNodeIDIndex := -1
			for i, vdr := range partition.validators {
				if vdr.NodeID == nodeID {
					foundNodeIDIndex = i
					break
				}
			}
			r.Equal(test.expectedIndex, foundNodeIDIndex)
			r.Equal(partition.validators[test.expectedIndex].NodeID, nodeID)
		})
	}
}

// Running tool: /usr/local/go/bin/go test -benchmem -run=^$ -bench ^BenchmarkAssignTx$ github.com/ava-labs/hypersdk/x/fortification -timeout=15s
//
// goos: darwin
// goarch: arm64
// pkg: github.com/ava-labs/hypersdk/x/fortification
// BenchmarkAssignTx/Vdrs-10-Txs-10-12         	  353709	      3374 ns/op	     896 B/op	       1 allocs/op
// BenchmarkAssignTx/Vdrs-10-Txs-100-12        	   36574	     32835 ns/op	    8216 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-10-Txs-1000-12       	    3820	    321792 ns/op	  122905 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-10-Txs-10000-12      	     372	   3196143 ns/op	  958494 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-100-Txs-10-12        	  342835	      3449 ns/op	     906 B/op	       1 allocs/op
// BenchmarkAssignTx/Vdrs-100-Txs-100-12       	   35043	     34827 ns/op	    8226 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-100-Txs-1000-12      	    3562	    327438 ns/op	  122904 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-100-Txs-10000-12     	     374	   3184245 ns/op	  958500 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-1000-Txs-10-12       	  345256	      3491 ns/op	     906 B/op	       1 allocs/op
// BenchmarkAssignTx/Vdrs-1000-Txs-100-12      	   33282	     34873 ns/op	    8496 B/op	       5 allocs/op
// BenchmarkAssignTx/Vdrs-1000-Txs-1000-12     	    3369	    356684 ns/op	  122910 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-1000-Txs-10000-12    	     330	   3452680 ns/op	  958492 B/op	       2 allocs/op
// BenchmarkAssignTx/Vdrs-10000-Txs-10-12      	  326670	      3721 ns/op	     906 B/op	       1 allocs/op
// BenchmarkAssignTx/Vdrs-10000-Txs-100-12     	   32966	     37805 ns/op	    8642 B/op	       6 allocs/op
// BenchmarkAssignTx/Vdrs-10000-Txs-1000-12    	    3120	    384614 ns/op	  122986 B/op	       5 allocs/op
// BenchmarkAssignTx/Vdrs-10000-Txs-10000-12   	     316	   4164289 ns/op	  958720 B/op	       7 allocs/op
// PASS
// ok  	github.com/ava-labs/hypersdk/x/fortification	22.653s
func BenchmarkAssignTx(b *testing.B) {
	vdrSetSizes := []int{10, 100, 1000, 10_000}
	txBatchSizes := []int{10, 100, 1000, 10_000}
	for _, numVdrs := range vdrSetSizes {
		// Create validator set with equal weights
		vdrWeights := make([]uint64, numVdrs)
		for i := range numVdrs {
			vdrWeights[i] = 100
		}
		partition := createTestPartition(vdrWeights)

		// Create txs once for largest batch size
		testTxs := make([]testPartitionTx, txBatchSizes[len(txBatchSizes)-1])
		for i := range testTxs {
			testTxs[i] = testPartitionTx(codec.CreateAddress(0, ids.GenerateTestID()))
		}

		for _, numTxs := range txBatchSizes {
			b.Run("Vdrs-"+strconv.Itoa(numVdrs)+"-Txs-"+strconv.Itoa(numTxs), func(b *testing.B) {
				r := require.New(b)
				for n := 0; n < b.N; n++ {
					assignments := make(map[ids.NodeID]testPartitionTx, numTxs)
					for _, tx := range testTxs[:numTxs] {
						nodeID, ok := AssignTx(partition, tx)
						r.True(ok)
						assignments[nodeID] = tx
					}
				}
			})
		}
	}
}

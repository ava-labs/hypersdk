// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"encoding/binary"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/stretchr/testify/require"
	"github.com/thepudds/fzgen/fuzzer"
)

// createTestPartition creates a precalculated partition with the provided weights
// and generates sorted nodeIDs to match the original ordering of weights
func createTestPartition(weights []uint64) *PrecalculatedPartition[tx] {
	nodeIDs := make([]ids.NodeID, 0, len(weights))
	for i := 0; i < len(weights); i++ {
		nodeIDs = append(nodeIDs, ids.GenerateTestNodeID())
	}
	utils.Sort(nodeIDs)

	validators := make([]*weightedValidator, 0, len(weights))
	totalWeight := uint64(0)
	for i, weight := range weights {
		validators = append(validators, &weightedValidator{
			weight: weight,
			nodeID: nodeIDs[i],
		})
		totalWeight += weight
	}
	return &PrecalculatedPartition[tx]{
		validators:  validators,
		totalWeight: totalWeight,
	}
}

func createTestPartitionTx(weight uint64) tx {
	sponsorAddrID := ids.GenerateTestID()
	binary.BigEndian.PutUint64(
		sponsorAddrID[len(sponsorAddrID)-consts.Uint64Len:],
		weight,
	)

	sponsorAddr := codec.CreateAddress(0, sponsorAddrID)
	return tx{
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

func createFuzzPartitionAndTxs(fz *fuzzer.Fuzzer, numVdrs int, numTxs int) (*PrecalculatedPartition[tx], []tx, bool) {
	vdrs := make([]uint64, 0, numVdrs)
	for i := 0; i < numVdrs; i++ {
		vdrWeight := uint64(0)
		fz.Fill(&vdrWeight)
		if vdrWeight == 0 {
			vdrWeight = 1
		}
		vdrs = append(vdrs, vdrWeight)
	}

	sum := uint64(0)
	for _, vdrWeight := range vdrs {
		updatedSum, err := math.Add(sum, vdrWeight)
		if err != nil {
			return nil, nil, false
		}
		sum = updatedSum
	}

	txs := make([]tx, 0, numTxs)
	for i := 0; i < numTxs; i++ {
		txWeight := uint64(0)
		fz.Fill(&txWeight)
		txs = append(txs, createTestPartitionTx(txWeight))
	}

	return createTestPartition(vdrs), txs, true
}

func FuzzAssignTxs(f *testing.F) {
	for i := 0; i < 100; i++ {
		f.Add([]byte{byte(i)})
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		fz := fuzzer.NewFuzzer(data)
		partition, txs, ok := createFuzzPartitionAndTxs(fz, 4, 4)
		if !ok {
			return
		}

		r := require.New(t)
		assignedTxs, err := partition.AssignTxs(txs)
		r.NoError(err)

		expectedAssignments := make(map[ids.NodeID][]tx)
		for _, tx := range txs {
			nodeID, ok := partition.AssignTx(tx)
			r.True(ok)
			expectedAssignments[nodeID] = append(expectedAssignments[nodeID], tx)
		}
		r.Equal(len(expectedAssignments), len(assignedTxs))
		for nodeID, txs := range assignedTxs {
			expectedTxs, ok := expectedAssignments[nodeID]
			r.True(ok)
			r.ElementsMatch(expectedTxs, txs)
		}
	})
}

func FuzzFilterTxs(f *testing.F) {
	for i := 0; i < 100; i++ {
		f.Add([]byte{byte(i)})
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		fz := fuzzer.NewFuzzer(data)
		partition, txs, ok := createFuzzPartitionAndTxs(fz, 4, 4)
		if !ok {
			return
		}

		nodeIDIndex := 0
		fz.Fill(&nodeIDIndex)
		nodeIDIndex = nodeIDIndex % len(partition.validators)
		nodeID := partition.validators[nodeIDIndex].nodeID

		r := require.New(t)
		filteredTxs, err := partition.FilterTxs(nodeID, txs)
		r.NoError(err)
		filteredTxsSet := make(map[ids.ID]struct{})
		for _, tx := range filteredTxs {
			filteredTxsSet[tx.ID] = struct{}{}
		}

		for _, tx := range txs {
			nodeID, ok := partition.AssignTx(tx)
			r.True(ok)
			expectedIncluded := nodeID == nodeID
			_, included := filteredTxsSet[tx.ID]
			r.Equal(expectedIncluded, included)
		}
	})
}

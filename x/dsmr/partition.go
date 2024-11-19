// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/cache"
)

const partitionCacheSize = 100

type weightedValidator struct {
	weight uint64
	nodeID ids.NodeID
}

type PrecalculatedPartition[T Tx] struct {
	validators  []*weightedValidator
	totalWeight uint64
}

func (w *weightedValidator) Compare(o *weightedValidator) int {
	return bytes.Compare(w.nodeID[:], o.nodeID[:])
}

func CalculatePartition[T Tx](ctx context.Context, state validators.State, pChainHeight uint64, subnetID ids.ID) (*PrecalculatedPartition[T], error) {
	vdrs, err := state.GetValidatorSet(ctx, pChainHeight, subnetID)
	if err != nil {
		return nil, err
	}
	weightedValidators := make([]*weightedValidator, 0, len(vdrs))
	totalWeight := uint64(0)
	for _, vdr := range vdrs {
		weightedValidators = append(weightedValidators, &weightedValidator{
			weight: vdr.Weight,
			nodeID: vdr.NodeID,
		})
		totalWeight += vdr.Weight
	}
	utils.Sort(weightedValidators)

	return &PrecalculatedPartition[T]{
		validators:  weightedValidators,
		totalWeight: totalWeight,
	}, nil
}

func calculateSponsorWeightIndex(sponsor codec.Address, totalWeight uint64) uint64 {
	return binary.BigEndian.Uint64(sponsor[len(sponsor)-consts.Uint64Len:]) % totalWeight
}

func (pp *PrecalculatedPartition[T]) AssignTx(tx T) (ids.NodeID, bool) {
	sponsor := tx.GetSponsor()
	sponsorWeightIndex := calculateSponsorWeightIndex(sponsor, pp.totalWeight)
	weightIndex := uint64(0)
	for _, vdr := range pp.validators {
		weightIndex += vdr.weight
		if weightIndex > sponsorWeightIndex {
			return vdr.nodeID, true
		}
	}
	// This should only happen if there are no validators
	return ids.NodeID{}, false
}

type indexedTx[T Tx] struct {
	index uint64
	tx    T
}

func (i *indexedTx[T]) Compare(o *indexedTx[T]) int {
	switch {
	case i.index < o.index:
		return -1
	case i.index > o.index:
		return 1
	default:
		return 0
	}
}

func (pp *PrecalculatedPartition[T]) calculateIndexedTxs(txs []T) []*indexedTx[T] {
	indexedTxs := make([]*indexedTx[T], len(txs))
	for i, tx := range txs {
		sponsor := tx.GetSponsor()
		sponsorWeightIndex := calculateSponsorWeightIndex(sponsor, pp.totalWeight)
		indexedTxs[i] = &indexedTx[T]{
			index: sponsorWeightIndex,
			tx:    tx,
		}
	}
	utils.Sort(indexedTxs)
	return indexedTxs
}

func (pp *PrecalculatedPartition[T]) AssignTxs(txs []T) (map[ids.NodeID][]T, error) {
	indexedTxs := pp.calculateIndexedTxs(txs)

	assignments := make(map[ids.NodeID][]T)
	weightIndex := uint64(0)
	txIndex := 0
	for _, vdr := range pp.validators {
		weightIndex += vdr.weight
		for txIndex < len(indexedTxs) && weightIndex > indexedTxs[txIndex].index {
			assignments[vdr.nodeID] = append(assignments[vdr.nodeID], indexedTxs[txIndex].tx)
			txIndex++
		}
		if txIndex >= len(indexedTxs) {
			break
		}
	}
	return assignments, nil
}

func (pp *PrecalculatedPartition[T]) FilterTxs(nodeID ids.NodeID, txs []T) ([]T, error) {
	// Return early if there are no transactions to filter
	if len(txs) == 0 {
		return nil, nil
	}

	// Calculate the weight index boundaries for the provided nodeID
	weightIndex := uint64(0)
	startWeight := uint64(0)
	endWeight := uint64(0)
	for _, vdr := range pp.validators {
		if vdr.nodeID == nodeID {
			startWeight = weightIndex
			endWeight = weightIndex + vdr.weight
			break
		}
		weightIndex += vdr.weight
	}

	filteredTxs := make([]T, 0, len(txs)/2)
	for _, tx := range txs {
		txIndexWeight := calculateSponsorWeightIndex(tx.GetSponsor(), pp.totalWeight)
		if endWeight > txIndexWeight && startWeight <= txIndexWeight {
			filteredTxs = append(filteredTxs, tx)
		}
	}

	return filteredTxs, nil
}

type Partition[T Tx] struct {
	state    validators.State
	subnetID ids.ID

	partitions *cache.FIFO[uint64, *PrecalculatedPartition[T]]
}

func NewPartition[T Tx](state validators.State, subnetID ids.ID) (*Partition[T], error) {
	partitionCache, err := cache.NewFIFO[uint64, *PrecalculatedPartition[T]](partitionCacheSize)
	if err != nil {
		return nil, err
	}
	return &Partition[T]{
		state:      state,
		subnetID:   subnetID,
		partitions: partitionCache,
	}, nil
}

func (p *Partition[T]) GetPartition(ctx context.Context, pChainHeight uint64) (*PrecalculatedPartition[T], error) {
	partition, ok := p.partitions.Get(pChainHeight)
	if ok {
		return partition, nil
	}

	partition, err := CalculatePartition[T](ctx, p.state, pChainHeight, p.subnetID)
	if err != nil {
		return nil, err
	}
	p.partitions.Put(pChainHeight, partition)
	return partition, nil
}

func (p *Partition[T]) GetCurrentPartition(ctx context.Context) (*PrecalculatedPartition[T], error) {
	pChainHeight, err := p.state.GetCurrentHeight(ctx)
	if err != nil {
		return nil, err
	}

	return p.GetPartition(ctx, pChainHeight)
}

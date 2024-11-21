// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"bytes"
	"context"
	"encoding/binary"
	"sort"

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
	// set accumulatedWeight up to index in precomputePartition
	accumulatedWeight uint64
}

type PrecalculatedPartition[T Tx] struct {
	validators  []*weightedValidator
	totalWeight uint64
}

func (w *weightedValidator) Compare(o *weightedValidator) int {
	return bytes.Compare(w.nodeID[:], o.nodeID[:])
}

func getWeightedVdrsFromState(ctx context.Context, state validators.State, pChainHeight uint64, subnetID ids.ID) ([]*weightedValidator, error) {
	vdrs, err := state.GetValidatorSet(ctx, pChainHeight, subnetID)
	if err != nil {
		return nil, err
	}
	weightedValidators := make([]*weightedValidator, 0, len(vdrs))
	for _, vdr := range vdrs {
		weightedValidators = append(weightedValidators, &weightedValidator{
			weight: vdr.Weight,
			nodeID: vdr.NodeID,
		})
	}
	return weightedValidators, nil
}

func precomputePartition[T Tx](validators []*weightedValidator) *PrecalculatedPartition[T] {
	utils.Sort(validators)
	accumulatedWeight := uint64(0)
	for _, weightedVdr := range validators {
		accumulatedWeight += weightedVdr.weight
		weightedVdr.accumulatedWeight = accumulatedWeight
	}
	return &PrecalculatedPartition[T]{
		validators:  validators,
		totalWeight: accumulatedWeight,
	}
}

func CalculatePartition[T Tx](ctx context.Context, state validators.State, pChainHeight uint64, subnetID ids.ID) (*PrecalculatedPartition[T], error) {
	weightedValidators, err := getWeightedVdrsFromState(ctx, state, pChainHeight, subnetID)
	if err != nil {
		return nil, err
	}

	return precomputePartition[T](weightedValidators), nil
}

func calculateSponsorWeightIndex(sponsor codec.Address, totalWeight uint64) uint64 {
	return binary.BigEndian.Uint64(sponsor[len(sponsor)-consts.Uint64Len:]) % totalWeight
}

func (pp *PrecalculatedPartition[T]) AssignTx(tx T) (ids.NodeID, bool) {
	if pp.totalWeight == 0 || len(pp.validators) == 0 {
		return ids.NodeID{}, false
	}

	sponsor := tx.GetSponsor()
	sponsorWeightIndex := calculateSponsorWeightIndex(sponsor, pp.totalWeight)
	nodeIDIndex := sort.Search(len(pp.validators), func(i int) bool {
		return pp.validators[i].accumulatedWeight > sponsorWeightIndex
	})

	// Defensive: this should never happen
	if nodeIDIndex >= len(pp.validators) {
		return ids.NodeID{}, false
	}
	return pp.validators[nodeIDIndex].nodeID, true
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

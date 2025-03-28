// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fortification

import (
	"bytes"
	"context"
	"encoding/binary"
	"sort"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/cache"
)

const partitionCacheSize = 100 // TODO: after epochs, we should only need to support two epochs.

type Tx interface {
	GetSponsor() codec.Address
}

type WeightedValidator struct {
	Weight uint64
	NodeID ids.NodeID
}

type Partition struct {
	validators         []WeightedValidator
	accumulatedWeights []uint64
	totalWeight        uint64
}

func (w WeightedValidator) Compare(o WeightedValidator) int {
	return bytes.Compare(w.NodeID[:], o.NodeID[:])
}

func NewPartition(weightedVdrs []WeightedValidator) *Partition {
	utils.Sort(weightedVdrs)
	accumulatedWeight := uint64(0)
	accumulatedWeights := make([]uint64, len(weightedVdrs))
	for i, weightedVdr := range weightedVdrs {
		accumulatedWeight += weightedVdr.Weight
		accumulatedWeights[i] = accumulatedWeight
		weightedVdrs[i] = weightedVdr
	}
	return &Partition{
		validators:         weightedVdrs,
		accumulatedWeights: accumulatedWeights,
		totalWeight:        accumulatedWeight,
	}
}

func calculateSponsorWeightIndex(sponsor codec.Address, totalWeight uint64) uint64 {
	return binary.BigEndian.Uint64(sponsor[len(sponsor)-consts.Uint64Len:]) % totalWeight
}

func AssignTx[T Tx](partition *Partition, tx T) (ids.NodeID, bool) {
	sponsor := tx.GetSponsor()
	sponsorWeightIndex := calculateSponsorWeightIndex(sponsor, partition.totalWeight)
	vdrIndex := sort.Search(len(partition.validators), func(i int) bool {
		return partition.accumulatedWeights[i] > sponsorWeightIndex
	})

	return partition.validators[vdrIndex].NodeID, true
}

type ChainState interface {
	// EstimatePChainHeight returns the estimated P-Chain height a correct node should use to build a new block
	EstimatePChainHeight(ctx context.Context) (uint64, error)
	// GetValidatorSet returns the validators of the provided subnet at the
	// requested P-chain height.
	// The returned map should not be modified.
	GetValidatorSet(
		ctx context.Context,
		height uint64,
		// subnetID ids.ID, TODO: we should not need to supply our own subnetID if we are converting to the interface we want
	) (map[ids.NodeID]*validators.GetValidatorOutput, error)
}

type TxPartition[T Tx] struct {
	log            logging.Logger
	chainState     ChainState
	partitionCache *cache.FIFO[uint64, *Partition]
}

func NewTxPartition[T Tx](log logging.Logger, chainState ChainState) (*TxPartition[T], error) {
	partitionCache, err := cache.NewFIFO[uint64, *Partition](partitionCacheSize)
	if err != nil {
		return nil, err
	}

	return &TxPartition[T]{
		log:            log,
		chainState:     chainState,
		partitionCache: partitionCache,
	}, nil
}

func (t *TxPartition[T]) getPartition(ctx context.Context) (*Partition, error) {
	pChainHeight, err := t.chainState.EstimatePChainHeight(ctx)
	if err != nil {
		return nil, err
	}
	partition, ok := t.partitionCache.Get(pChainHeight)
	if !ok {
		validatorSet, err := t.chainState.GetValidatorSet(ctx, pChainHeight)
		if err != nil {
			return nil, err
		}

		validators := make([]WeightedValidator, 0, len(validatorSet))
		for nodeID, validatorOutput := range validatorSet {
			validators = append(validators, WeightedValidator{
				Weight: validatorOutput.Weight,
				NodeID: nodeID,
			})
		}

		partition = NewPartition(validators)
		t.partitionCache.Put(pChainHeight, partition)
	}

	return partition, nil
}

func (t *TxPartition[T]) FilterTxs(ctx context.Context, nodeID ids.NodeID, txs []T) []T {
	partition, err := t.getPartition(ctx)
	if err != nil {
		t.log.Warn("unable to get partition", zap.Error(err))
		return nil
	}
	filteredTxs := make([]T, 0, len(txs))
	for _, tx := range txs {
		if nodeID, ok := AssignTx(partition, tx); ok && nodeID == nodeID {
			filteredTxs = append(filteredTxs, tx)
		}
	}

	return filteredTxs
}

func (t *TxPartition[T]) AssignTx(ctx context.Context, tx T) []ids.NodeID {
	partition, err := t.getPartition(ctx)
	if err != nil {
		t.log.Warn("unable to get partition", zap.Error(err))
		return nil
	}
	nodeID, ok := AssignTx(partition, tx)
	if !ok {
		t.log.Warn("unable to assign tx to node", zap.Any("tx", tx))
		return nil
	}
	return []ids.NodeID{nodeID}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"bytes"
	"encoding/binary"
	"sort"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

type weightedValidator struct {
	weight uint64
	nodeID ids.NodeID
	// set accumulatedWeight up to index in precomputePartition
	accumulatedWeight uint64
}

type Partition[T Tx] struct {
	validators  []weightedValidator
	totalWeight uint64
}

func (w weightedValidator) Compare(o weightedValidator) int {
	return bytes.Compare(w.nodeID[:], o.nodeID[:])
}

func NewPartition[T Tx](validators []Validator) *Partition[T] {
	weightedVdrs := make([]weightedValidator, len(validators))
	for i, vdr := range validators {
		weightedVdrs[i] = weightedValidator{
			weight: vdr.Weight,
			nodeID: vdr.NodeID,
		}
	}

	utils.Sort(weightedVdrs)
	accumulatedWeight := uint64(0)
	for i, weightedVdr := range weightedVdrs {
		accumulatedWeight += weightedVdr.weight
		weightedVdr.accumulatedWeight = accumulatedWeight
		weightedVdrs[i] = weightedVdr
	}
	return &Partition[T]{
		validators:  weightedVdrs,
		totalWeight: accumulatedWeight,
	}
}

func calculateSponsorWeightIndex(sponsor codec.Address, totalWeight uint64) uint64 {
	return binary.BigEndian.Uint64(sponsor[len(sponsor)-consts.Uint64Len:]) % totalWeight
}

func (p *Partition[T]) AssignTx(tx T) (ids.NodeID, bool) {
	sponsor := tx.GetSponsor()
	sponsorWeightIndex := calculateSponsorWeightIndex(sponsor, p.totalWeight)
	nodeIDIndex := sort.Search(len(p.validators), func(i int) bool {
		return p.validators[i].accumulatedWeight > sponsorWeightIndex
	})

	return p.validators[nodeIDIndex].nodeID, true
}

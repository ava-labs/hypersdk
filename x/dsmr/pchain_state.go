// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
)

var _ ChainState = (*PChainState)(nil)

var errNoValidators = errors.New("no validators")

// ChainState provides all information about the chain's validator set and P-Chain height (epoch)
// required by DSMR.
type ChainState interface {
	// EstimatePChainHeight returns the estimated P-Chain height a correct node should use to build a new block
	EstimatePChainHeight(ctx context.Context) (uint64, error)
	// GetCanonicalValidatorSet returns the canonically ordered validator set of the chain at the provided P-Chain height
	GetCanonicalValidatorSet(ctx context.Context, pChainHeight uint64) (validators warp.CanonicalValidatorSet, err error)
	// IsNodeValidator returns whether nodeID is a validator at the provided P-Chain height
	IsNodeValidator(ctx context.Context, nodeID ids.NodeID, pChainHeight uint64) (bool, error)
	// SampleNodeID returns a random nodeID from the current validator set
	SampleNodeID(ctx context.Context) (ids.NodeID, error)
}

// PChainState implements ChainState using the subnetID and validators.State provided via snow.Context to
// every Avalanche VM instance.
type PChainState struct {
	subnetID ids.ID
	vdrState validators.State
}

func NewPChainState(subnetID ids.ID, vdrState validators.State) *PChainState {
	return &PChainState{
		subnetID: subnetID,
		vdrState: vdrState,
	}
}

func (p *PChainState) EstimatePChainHeight(ctx context.Context) (uint64, error) {
	return p.vdrState.GetCurrentHeight(ctx)
}

func (p *PChainState) GetCanonicalValidatorSet(ctx context.Context, pChainHeight uint64) (validators warp.CanonicalValidatorSet, err error) {
	return warp.GetCanonicalValidatorSetFromSubnetID(ctx, p.vdrState, pChainHeight, p.subnetID)
}

func (p *PChainState) IsNodeValidator(ctx context.Context, nodeID ids.NodeID, pChainHeight uint64) (bool, error) {
	vdrSet, err := p.vdrState.GetValidatorSet(ctx, pChainHeight, p.subnetID)
	if err != nil {
		return false, err
	}
	_, ok := vdrSet[nodeID]
	return ok, nil
}

func (p *PChainState) SampleNodeID(ctx context.Context) (ids.NodeID, error) {
	estimatedPChainHeight, err := p.EstimatePChainHeight(ctx)
	if err != nil {
		return ids.EmptyNodeID, err
	}

	vdrSet, err := p.vdrState.GetValidatorSet(ctx, estimatedPChainHeight, p.subnetID)
	if err != nil {
		return ids.EmptyNodeID, err
	}
	for nodeID := range vdrSet {
		return nodeID, nil
	}
	return ids.EmptyNodeID, errNoValidators
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/x/dsmr"
	"github.com/ava-labs/hypersdk/x/fortification"
)

var (
	_ dsmr.ChainState          = (*PChainState)(nil)
	_ fortification.ChainState = (*PChainState)(nil)
)

var errNoValidators = errors.New("no validators")

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

// GetValidatorSet returns the validators of the provided subnet at the
// requested P-chain height.
// The returned map should not be modified.
func (p *PChainState) GetValidatorSet(
	ctx context.Context,
	height uint64,
) (map[ids.NodeID]*validators.GetValidatorOutput, error) {
	return p.vdrState.GetValidatorSet(ctx, height, p.subnetID)
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

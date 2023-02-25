// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/proposervm/proposer"
)

const (
	refreshTime            = 30 * time.Second
	proposerMonitorLRUSize = 60
)

type ProposerMonitor struct {
	vm       *VM
	proposer proposer.Windower

	currentPHeight     uint64
	lastFetchedPHeight time.Time
	validators         map[ids.NodeID]*validators.GetValidatorOutput

	proposerCache *cache.LRU[string, []ids.NodeID]

	rl sync.Mutex
}

func NewProposerMonitor(vm *VM) *ProposerMonitor {
	return &ProposerMonitor{
		vm: vm,
		proposer: proposer.New(
			vm.snowCtx.ValidatorState,
			vm.snowCtx.SubnetID,
			vm.snowCtx.ChainID,
		),
		proposerCache: &cache.LRU[string, []ids.NodeID]{Size: proposerMonitorLRUSize},
	}
}

func (p *ProposerMonitor) refresh(ctx context.Context) error {
	p.rl.Lock()
	defer p.rl.Unlock()

	// Refresh P-Chain height if [refreshTime] has elapsed
	if time.Since(p.lastFetchedPHeight) > refreshTime {
		pHeight, err := p.vm.snowCtx.ValidatorState.GetCurrentHeight(ctx)
		if err != nil {
			return err
		}
		p.validators, err = p.vm.snowCtx.ValidatorState.GetValidatorSet(
			ctx,
			pHeight,
			p.vm.snowCtx.SubnetID,
		)
		if err != nil {
			return err
		}
		p.currentPHeight = pHeight
		p.lastFetchedPHeight = time.Now()
	}
	return nil
}

func (p *ProposerMonitor) IsValidator(ctx context.Context, nodeID ids.NodeID) (bool, error) {
	if err := p.refresh(ctx); err != nil {
		return false, err
	}
	_, ok := p.validators[nodeID]
	return ok, nil
}

func (p *ProposerMonitor) Proposers(
	ctx context.Context,
	diff int,
	depth int,
) (set.Set[ids.NodeID], error) {
	if err := p.refresh(ctx); err != nil {
		return nil, err
	}
	preferredBlk, err := p.vm.GetStatelessBlock(ctx, p.vm.preferred)
	if err != nil {
		return nil, err
	}
	proposersToGossip := set.NewSet[ids.NodeID](diff * depth)
	udepth := uint64(depth)
	for i := uint64(1); i <= uint64(diff); i++ {
		height := preferredBlk.Hght + i
		key := fmt.Sprintf("%d-%d", height, p.currentPHeight)
		var proposers []ids.NodeID
		if v, ok := p.proposerCache.Get(key); ok {
			proposers = v
		} else {
			proposers, err = p.proposer.Proposers(ctx, height, p.currentPHeight)
			if err != nil {
				return nil, err
			}
			p.proposerCache.Put(key, proposers)
		}
		arrLen := math.Min(udepth, uint64(len(proposers)))
		proposersToGossip.Add(proposers[:arrLen]...)
	}
	return proposersToGossip, nil
}

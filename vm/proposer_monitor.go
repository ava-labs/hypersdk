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
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/avalanchego/vms/proposervm/proposer"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

const (
	refreshTime            = 30 * time.Second
	proposerMonitorLRUSize = 60
)

type ProposerMonitor struct {
	vm       *VM
	proposer proposer.Windower

	currentPHeight      uint64
	lastFetchedPHeight  time.Time
	validators          map[ids.NodeID]*validators.GetValidatorOutput
	validatorPublicKeys map[string]struct{}

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
	// Refresh P-Chain height if [refreshTime] has elapsed
	if time.Since(p.lastFetchedPHeight) < refreshTime {
		return nil
	}
	start := time.Now()
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
	pks := map[string]struct{}{}
	for _, v := range p.validators {
		if v.PublicKey == nil {
			continue
		}
		pks[string(bls.PublicKeyToBytes(v.PublicKey))] = struct{}{}
	}
	p.validatorPublicKeys = pks
	p.vm.snowCtx.Log.Info(
		"refreshed proposer monitor",
		zap.Uint64("previous", p.currentPHeight),
		zap.Uint64("new", pHeight),
		zap.Duration("t", time.Since(start)),
	)
	p.currentPHeight = pHeight
	p.lastFetchedPHeight = time.Now()
	return nil
}

func (p *ProposerMonitor) IsValidator(ctx context.Context, nodeID ids.NodeID) (bool, *bls.PublicKey, uint64, error) {
	p.rl.Lock()
	defer p.rl.Unlock()

	if err := p.refresh(ctx); err != nil {
		return false, nil, 0, err
	}
	output, ok := p.validators[nodeID]
	return ok, output.PublicKey, output.Weight, nil
}

func (p *ProposerMonitor) Proposers(
	ctx context.Context,
	diff int,
	depth int,
) (set.Set[ids.NodeID], error) {
	p.rl.Lock()
	defer p.rl.Unlock()

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
		height := preferredBlk.Height() + i
		key := fmt.Sprintf("%d-%d", height, p.currentPHeight)
		var proposers []ids.NodeID
		if v, ok := p.proposerCache.Get(key); ok {
			proposers = v
		} else {
			proposers, err = p.proposer.Proposers(ctx, height, p.currentPHeight, diff)
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

// getCanonicalValidatorSet returns the validator set of [subnetID] in a canonical ordering.
// Also returns the total weight on [subnetID].
func (p *ProposerMonitor) GetCanonicalValidatorSet(
	ctx context.Context,
) (uint64, []*warp.Validator, uint64, error) {
	p.rl.Lock()
	defer p.rl.Unlock()

	if err := p.refresh(ctx); err != nil {
		return 0, nil, 0, err
	}

	vdrList, totalWeight, err := utils.ConstructCanonicalValidatorSet(p.validators)
	if err != nil {
		return 0, nil, 0, err
	}
	return p.currentPHeight, vdrList, totalWeight, nil
}

func (p *ProposerMonitor) GetValidatorSet(
	ctx context.Context,
	includeMe bool,
) (set.Set[ids.NodeID], error) {
	p.rl.Lock()
	defer p.rl.Unlock()

	if err := p.refresh(ctx); err != nil {
		return nil, err
	}

	vdrSet := set.NewSet[ids.NodeID](len(p.validators))
	for v := range p.validators {
		if v == p.vm.snowCtx.NodeID && !includeMe {
			continue
		}
		vdrSet.Add(v)
	}
	return vdrSet, nil
}

// Prevent unnecessary map copies
func (p *ProposerMonitor) IterateValidators(
	ctx context.Context,
	f func(ids.NodeID, *validators.GetValidatorOutput),
) error {
	p.rl.Lock()
	defer p.rl.Unlock()

	if err := p.refresh(ctx); err != nil {
		return err
	}

	for k, v := range p.validators {
		f(k, v)
	}
	return nil
}

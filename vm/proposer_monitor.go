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
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/avalanchego/vms/proposervm/proposer"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
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
	p.rl.Lock()
	defer p.rl.Unlock()

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

// TODO: remove this function, isn't safe?
func (p *ProposerMonitor) Validators(
	ctx context.Context,
) (map[ids.NodeID]*validators.GetValidatorOutput, map[string]struct{}) {
	if err := p.refresh(ctx); err != nil {
		// TODO: return error
		return nil, nil
	}
	return p.validators, p.validatorPublicKeys
}

// getCanonicalValidatorSet returns the validator set of [subnetID] in a canonical ordering.
// Also returns the total weight on [subnetID].
func (p *ProposerMonitor) GetCanonicalValidatorSet(
	ctx context.Context,
) (uint64, []*warp.Validator, uint64, error) {
	if err := p.refresh(ctx); err != nil {
		return 0, nil, 0, err
	}

	var (
		vdrs        = make(map[string]*warp.Validator, len(p.validators))
		totalWeight uint64
		err         error
	)
	for _, vdr := range p.validators {
		totalWeight, err = math.Add64(totalWeight, vdr.Weight)
		if err != nil {
			return 0, nil, 0, fmt.Errorf("%w: %v", warp.ErrWeightOverflow, err) //nolint:errorlint
		}

		if vdr.PublicKey == nil {
			fmt.Println("skipping validator because of empty public key", vdr.NodeID)
			continue
		}

		pkBytes := bls.PublicKeyToBytes(vdr.PublicKey)
		uniqueVdr, ok := vdrs[string(pkBytes)]
		if !ok {
			uniqueVdr = &warp.Validator{
				PublicKey:      vdr.PublicKey,
				PublicKeyBytes: pkBytes,
			}
			vdrs[string(pkBytes)] = uniqueVdr
		}

		uniqueVdr.Weight += vdr.Weight // Impossible to overflow here
		uniqueVdr.NodeIDs = append(uniqueVdr.NodeIDs, vdr.NodeID)
	}

	// Sort validators by public key
	vdrList := maps.Values(vdrs)
	utils.Sort(vdrList)
	return p.currentPHeight, vdrList, totalWeight, nil
}

func (p *ProposerMonitor) GetValidatorSet(
	ctx context.Context,
	includeMe bool,
) set.Set[ids.NodeID] {
	vdrs, _ := p.Validators(ctx)
	vdrSet := set.NewSet[ids.NodeID](len(vdrs))
	for v := range vdrs {
		if v == p.vm.snowCtx.NodeID && !includeMe {
			continue
		}
		vdrSet.Add(v)
	}
	return vdrSet
}

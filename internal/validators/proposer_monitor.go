// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/proposervm/proposer"
	"go.uber.org/zap"
)

const (
	refreshTime            = 30 * time.Second
	proposerMonitorLRUSize = 60
)

type Backend interface {
	PreferredHeight(ctx context.Context) (uint64, error)
}

type ProposerMonitor struct {
	backend  Backend
	snowCtx  *snow.Context
	proposer proposer.Windower

	currentPHeight      uint64
	lastFetchedPHeight  time.Time
	validators          map[ids.NodeID]*validators.GetValidatorOutput
	validatorPublicKeys map[string]struct{}

	proposerCache *cache.LRU[string, []ids.NodeID]

	rl sync.Mutex
}

func NewProposerMonitor(backend Backend, snowCtx *snow.Context) *ProposerMonitor {
	return &ProposerMonitor{
		backend: backend,
		snowCtx: snowCtx,
		proposer: proposer.New(
			snowCtx.ValidatorState,
			snowCtx.SubnetID,
			snowCtx.ChainID,
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
	pHeight, err := p.snowCtx.ValidatorState.GetCurrentHeight(ctx)
	if err != nil {
		return err
	}
	p.validators, err = p.snowCtx.ValidatorState.GetValidatorSet(
		ctx,
		pHeight,
		p.snowCtx.SubnetID,
	)
	if err != nil {
		return err
	}
	pks := map[string]struct{}{}
	for _, v := range p.validators {
		if v.PublicKey == nil {
			continue
		}
		pks[string(bls.PublicKeyToCompressedBytes(v.PublicKey))] = struct{}{}
	}
	p.validatorPublicKeys = pks
	p.snowCtx.Log.Info(
		"refreshed proposer monitor",
		zap.Uint64("previous", p.currentPHeight),
		zap.Uint64("new", pHeight),
		zap.Duration("t", time.Since(start)),
	)
	p.currentPHeight = pHeight
	p.lastFetchedPHeight = time.Now()
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
	preferredHeight, err := p.backend.PreferredHeight(ctx)
	if err != nil {
		return nil, err
	}
	proposersToGossip := set.NewSet[ids.NodeID](diff * depth)
	udepth := uint64(depth)
	for i := uint64(1); i <= uint64(diff); i++ {
		height := preferredHeight + i
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
		arrLen := min(udepth, uint64(len(proposers)))
		proposersToGossip.Add(proposers[:arrLen]...)
	}
	return proposersToGossip, nil
}

func (p *ProposerMonitor) Validators(
	ctx context.Context,
) (map[ids.NodeID]*validators.GetValidatorOutput, map[string]struct{}) {
	if err := p.refresh(ctx); err != nil {
		return nil, nil
	}
	return p.validators, p.validatorPublicKeys
}

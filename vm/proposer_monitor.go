// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

const (
	refreshTime            = 30 * time.Second
	proposerMonitorLRUSize = 60
)

type proposerInfo struct {
	validators   map[ids.NodeID]*validators.GetValidatorOutput
	partitionSet []ids.NodeID
	warpSet      []*warp.Validator
	totalWeight  uint64
}

type ProposerMonitor struct {
	vm *VM

	fetchLock sync.Mutex
	proposers *cache.LRU[uint64, *proposerInfo] // safe for concurrent use

	currentLock        sync.Mutex
	lastFetchedPHeight time.Time
	currentValidators  map[ids.NodeID]*validators.GetValidatorOutput
}

func NewProposerMonitor(vm *VM) *ProposerMonitor {
	return &ProposerMonitor{
		vm:        vm,
		proposers: &cache.LRU[uint64, *proposerInfo]{Size: proposerMonitorLRUSize},
	}
}

func (p *ProposerMonitor) fetch(ctx context.Context, height uint64) *proposerInfo {
	validators, err := p.vm.snowCtx.ValidatorState.GetValidatorSet(
		ctx,
		height,
		p.vm.snowCtx.SubnetID,
	)
	if err != nil {
		p.vm.snowCtx.Log.Error("failed to fetch proposer set", zap.Uint64("height", height), zap.Error(err))
		return nil
	}
	partitionSet, warpSet, totalWeight, err := utils.ConstructCanonicalValidatorSet(validators)
	if err != nil {
		p.vm.snowCtx.Log.Error("failed to construct canonical validator set", zap.Uint64("height", height), zap.Error(err))
		return nil
	}
	info := &proposerInfo{
		validators:   validators,
		partitionSet: partitionSet,
		warpSet:      warpSet,
		totalWeight:  totalWeight,
	}
	p.proposers.Put(height, info)
	return info
}

// Fetch is used to pre-cache sets that will be used later
func (p *ProposerMonitor) Fetch(ctx context.Context, height uint64) *proposerInfo {
	p.fetchLock.Lock()
	defer p.fetchLock.Unlock()

	return p.fetch(ctx, height)
}

func (p *ProposerMonitor) IsValidator(ctx context.Context, height uint64, nodeID ids.NodeID) (bool, *bls.PublicKey, uint64, error) {
	p.fetchLock.Lock()
	defer p.fetchLock.Unlock()

	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return false, nil, 0, errors.New("could not get validator set for height")
	}
	output, exists := info.validators[nodeID]
	return exists, output.PublicKey, output.Weight, nil
}

// GetWarpValidatorSet returns the validator set of [subnetID] in a canonical ordering.
// Also returns the total weight on [subnetID].
func (p *ProposerMonitor) GetWarpValidatorSet(ctx context.Context, height uint64) ([]*warp.Validator, uint64, error) {
	p.fetchLock.Lock()
	defer p.fetchLock.Unlock()

	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return nil, 0, errors.New("could not get validator set for height")
	}
	return info.warpSet, info.totalWeight, nil
}

func (p *ProposerMonitor) GetValidatorSet(ctx context.Context, height uint64, includeMe bool) (set.Set[ids.NodeID], error) {
	p.fetchLock.Lock()
	defer p.fetchLock.Unlock()

	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return nil, errors.New("could not get validator set for height")
	}
	vdrSet := set.NewSet[ids.NodeID](len(info.validators))
	for v := range info.validators {
		if v == p.vm.snowCtx.NodeID && !includeMe {
			continue
		}
		vdrSet.Add(v)
	}
	return vdrSet, nil
}

// Prevent unnecessary map copies
func (p *ProposerMonitor) IterateCurrentValidators(
	ctx context.Context,
	f func(ids.NodeID, *validators.GetValidatorOutput),
) error {
	// Refresh P-Chain height if [refreshTime] has elapsed
	p.currentLock.Lock()
	if time.Since(p.lastFetchedPHeight) > refreshTime {
		pHeight, err := p.vm.snowCtx.ValidatorState.GetCurrentHeight(ctx)
		if err != nil {
			p.currentLock.Unlock()
			return err
		}
		validators, err := p.vm.snowCtx.ValidatorState.GetValidatorSet(
			ctx,
			pHeight,
			p.vm.snowCtx.SubnetID,
		)
		if err != nil {
			p.currentLock.Unlock()
			return err
		}
		p.lastFetchedPHeight = time.Now()
		p.currentValidators = validators
	}
	validators := p.currentValidators
	p.currentLock.Unlock()

	// Iterate over the validators
	for k, v := range validators {
		f(k, v)
	}
	return nil
}

// Prevent unnecessary map copies
func (p *ProposerMonitor) IterateValidators(
	ctx context.Context,
	height uint64,
	f func(ids.NodeID, *validators.GetValidatorOutput),
) error {
	p.fetchLock.Lock()
	defer p.fetchLock.Unlock()

	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return errors.New("could not get validator set for height")
	}
	for k, v := range info.validators {
		f(k, v)
	}
	return nil
}

func (p *ProposerMonitor) RandomValidator(ctx context.Context, height uint64) (ids.NodeID, error) {
	p.fetchLock.Lock()
	defer p.fetchLock.Unlock()

	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return ids.NodeID{}, errors.New("could not get validator set for height")
	}
	for k := range info.validators { // Golang map iteration order is random
		return k, nil
	}
	return ids.NodeID{}, fmt.Errorf("no validators")
}

func (p *ProposerMonitor) AddressPartition(ctx context.Context, height uint64, addr codec.Address) (ids.NodeID, error) {
	p.fetchLock.Lock()
	defer p.fetchLock.Unlock()

	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return ids.NodeID{}, errors.New("could not get validator set for height")
	}
	if len(info.partitionSet) == 0 {
		return ids.NodeID{}, errors.New("no validators")
	}
	seed := make([]byte, consts.Uint64Len+codec.AddressLen)
	binary.BigEndian.PutUint64(seed, height)
	copy(seed[consts.Uint64Len:], addr[:])
	h := utils.ToID(seed)
	index := new(big.Int).Mod(new(big.Int).SetBytes(h[:]), big.NewInt(int64(len(info.partitionSet))))
	indexInt := int(index.Int64())
	p.vm.Logger().Info(
		"address partition computed",
		zap.Uint64("height", height),
		zap.String("address", hex.EncodeToString(addr[:])),
		zap.Int("index", indexInt),
	)
	return info.partitionSet[indexInt], nil
}

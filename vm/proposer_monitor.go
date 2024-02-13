// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/binary"
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
	aggrPubKeyLRUSize      = 1024
)

type proposerInfo struct {
	validators   map[ids.NodeID]*validators.GetValidatorOutput
	partitionSet []ids.NodeID
	warpSet      []*warp.Validator
	totalWeight  uint64
}

// TODO: change to PChainManager (or something like it)
type ProposerMonitor struct {
	vm *VM

	fetchLock sync.Mutex
	proposers *cache.LRU[uint64, *proposerInfo] // safe for concurrent use

	currentLock        sync.Mutex
	lastFetchedPHeight time.Time
	currentHeight      uint64
	currentValidators  map[ids.NodeID]*validators.GetValidatorOutput

	aggrCache *cache.LRU[string, *bls.PublicKey]
}

func NewProposerMonitor(vm *VM) *ProposerMonitor {
	return &ProposerMonitor{
		vm:        vm,
		proposers: &cache.LRU[uint64, *proposerInfo]{Size: proposerMonitorLRUSize},
		aggrCache: &cache.LRU[string, *bls.PublicKey]{Size: aggrPubKeyLRUSize},
	}
}

// TODO: don't add validators that won't be validators for the entire epoch
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

func (p *ProposerMonitor) refreshCurrent(ctx context.Context) error {
	pHeight, err := p.vm.snowCtx.ValidatorState.GetCurrentHeight(ctx)
	if err != nil {
		p.currentLock.Unlock()
		return err
	}
	p.currentHeight = pHeight
	validators, err := p.vm.snowCtx.ValidatorState.GetValidatorSet(
		ctx,
		pHeight,
		p.vm.snowCtx.SubnetID,
	)
	if err != nil {
		return err
	}
	p.lastFetchedPHeight = time.Now()
	p.currentValidators = validators
	return nil
}

// Prevent unnecessary map copies
func (p *ProposerMonitor) IterateCurrentValidators(
	ctx context.Context,
	f func(ids.NodeID, *validators.GetValidatorOutput),
) error {
	// Refresh P-Chain height if [refreshTime] has elapsed
	p.currentLock.Lock()
	if time.Since(p.lastFetchedPHeight) > refreshTime {
		if err := p.refreshCurrent(ctx); err != nil {
			p.currentLock.Unlock()
			return err
		}
	}
	validators := p.currentValidators
	p.currentLock.Unlock()

	// Iterate over the validators
	for k, v := range validators {
		f(k, v)
	}
	return nil
}

func (p *ProposerMonitor) IsValidHeight(ctx context.Context, height uint64) (bool, error) {
	p.currentLock.Lock()
	defer p.currentLock.Unlock()

	if height <= p.currentHeight {
		return true, nil
	}
	if err := p.refreshCurrent(ctx); err != nil {
		return false, err
	}
	return height <= p.currentHeight, nil
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
	return info.partitionSet[indexInt], nil
}

func (p *ProposerMonitor) GetAggregatePublicKey(ctx context.Context, height uint64, signers set.Bits, num, denom uint64) (*bls.PublicKey, error) {
	// Confirm signing weight is sufficient
	vdrSet, totalWeight, err := p.GetWarpValidatorSet(ctx, height)
	if err != nil {
		return nil, err
	}
	filtered, err := warp.FilterValidators(signers, vdrSet)
	if err != nil {
		return nil, err
	}
	filteredWeight, err := warp.SumWeight(filtered)
	if err != nil {
		return nil, err
	}
	if err := warp.VerifyWeight(filteredWeight, totalWeight, num, denom); err != nil {
		return nil, err
	}

	// Attempt to load aggregate public key from cache
	bitSetBytes := signers.Bytes()
	k := make([]byte, consts.Uint64Len+len(bitSetBytes))
	binary.BigEndian.PutUint64(k, height)
	copy(k[consts.Uint64Len:], bitSetBytes)
	sk := string(k)
	v, ok := p.aggrCache.Get(sk) // we may perform duplicate aggregations because we don't lock here
	if !ok {
		aggrPubKey, err := warp.AggregatePublicKeys(filtered)
		if err != nil {
			return nil, err
		}
		p.aggrCache.Put(sk, aggrPubKey)
		v = aggrPubKey
		p.vm.Logger().Info("caching aggregate public key", zap.Uint64("height", height), zap.String("signers", signers.String()))
	} else {
		p.vm.Logger().Info("found cached aggregate public key", zap.Uint64("height", height), zap.String("signers", signers.String()))
	}
	return v, nil
}

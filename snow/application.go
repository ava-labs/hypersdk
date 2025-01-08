// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"net/http"

	"github.com/ava-labs/avalanchego/api/health"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"

	"github.com/ava-labs/hypersdk/event"
)

// VM exposes the VM to the implementation of Chain
type VM[I Block, O Block, A Block] struct {
	vm *vm[I, O, A]
}

func (v *VM[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (I, error) {
	blk, err := v.vm.GetBlock(ctx, blkID)
	if err != nil {
		var emptyI I
		return emptyI, err
	}
	return blk.Input, nil
}

func (v *VM[I, O, A]) GetBlockByHeight(ctx context.Context, height uint64) (I, error) {
	blk, err := v.vm.GetBlockByHeight(ctx, height)
	if err != nil {
		var emptyI I
		return emptyI, err
	}
	return blk.Input, nil
}

func (v *VM[I, O, A]) ParseBlock(ctx context.Context, bytes []byte) (I, error) {
	blk, err := v.vm.ParseBlock(ctx, bytes)
	if err != nil {
		var emptyI I
		return emptyI, err
	}
	return blk.Input, nil
}

func (v *VM[I, O, A]) GetNetwork() *p2p.Network {
	return v.vm.network
}

func (v *VM[I, O, A]) LastAcceptedBlock(ctx context.Context) I {
	return v.vm.LastAcceptedBlock(ctx).Input
}

func (v *VM[I, O, A]) AddAcceptedSub(sub ...event.Subscription[A]) {
	v.vm.acceptedSubs = append(v.vm.acceptedSubs, sub...)
}

func (v *VM[I, O, A]) AddRejectedSub(sub ...event.Subscription[O]) {
	v.vm.rejectedSubs = append(v.vm.rejectedSubs, sub...)
}

func (v *VM[I, O, A]) AddVerifiedSub(sub ...event.Subscription[O]) {
	v.vm.verifiedSubs = append(v.vm.verifiedSubs, sub...)
}

func (v *VM[I, O, A]) AddPreReadyAcceptedSub(sub ...event.Subscription[I]) {
	v.vm.preReadyAcceptedSubs = append(v.vm.preReadyAcceptedSubs, sub...)
}

func (v *VM[I, O, A]) AddHandler(name string, handler http.Handler) {
	v.vm.handlers[name] = handler
}

func (v *VM[I, O, A]) AddHealthCheck(healthChecker health.Checker) {
	v.vm.healthChecker = healthChecker
}

func (v *VM[I, O, A]) AddCloser(name string, closer func() error) {
	v.vm.closers = append(v.vm.closers, namedCloser{name, closer})
}

func (v *VM[I, O, A]) AddStateSyncStarter(onStateSyncStarted ...func(context.Context) error) {
	v.vm.onStateSyncStarted = append(v.vm.onStateSyncStarted, onStateSyncStarted...)
}

func (v *VM[I, O, A]) AddNormalOpStarter(onNormalOpStartedF ...func(context.Context) error) {
	v.vm.onNormalOperationsStarted = append(v.vm.onNormalOperationsStarted, onNormalOpStartedF...)
}

// StartStateSync notifies the VM to enter DynamicStateSync mode.
// The caller is responsible to eventually call FinishStateSync with a fully populated
// last accepted state.
func (v *VM[I, O, A]) StartStateSync(ctx context.Context, block I) error {
	return v.vm.startStateSync(ctx, block)
}

// FinishStateSync completes dynamic state sync mode and sets the last accepted block to
// the given input/output/accepted value.
func (v *VM[I, O, A]) FinishStateSync(ctx context.Context, input I, output O, accepted A) error {
	return v.vm.finishStateSync(ctx, input, output, accepted)
}

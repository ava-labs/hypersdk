// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"net/http"

	"github.com/ava-labs/avalanchego/api/health"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"

	"github.com/ava-labs/hypersdk/event"
)

type Application[I Block, O Block] struct {
	vm *VM[I, O]

	Version         string
	Handlers        map[string]http.Handler
	HealthChecker   health.Checker
	Network         *p2p.Network
	StateSyncableVM block.StateSyncableVM
	Closers         []func() error

	OnStateSyncStarted       []func(context.Context) error
	OnBootstrapStarted       []func(context.Context) error
	OnNormalOperationStarted []func(context.Context) error

	VerifiedSubs         []event.Subscription[O]
	RejectedSubs         []event.Subscription[O]
	AcceptedSubs         []event.Subscription[O]
	PreReadyAcceptedSubs []event.Subscription[I]
}

// GetCovariantVM returns the VM implementation returning the wrapper around the generic types
func (a *Application[I, O]) GetCovariantVM() *CovariantVM[I, O] {
	return a.vm.covariantVM
}

// GetInputCovariantVM returns the VM implementation that returns the wrapper around the generic
// types (instead of the snowman.Block type)
func (a *Application[I, O]) GetInputCovariantVM() *InputCovariantVM[I, O] {
	return &InputCovariantVM[I, O]{a.vm.covariantVM}
}

func (a *Application[I, O]) WithAcceptedSub(sub ...event.Subscription[O]) {
	a.AcceptedSubs = append(a.AcceptedSubs, sub...)
}

func (a *Application[I, O]) WithRejectedSub(sub ...event.Subscription[O]) {
	a.RejectedSubs = append(a.RejectedSubs, sub...)
}

func (a *Application[I, O]) WithVerifiedSub(sub ...event.Subscription[O]) {
	a.VerifiedSubs = append(a.VerifiedSubs, sub...)
}

func (a *Application[I, O]) WithPreReadyAcceptedSub(sub ...event.Subscription[I]) {
	a.PreReadyAcceptedSubs = append(a.PreReadyAcceptedSubs, sub...)
}

func (a *Application[I, O]) WithHandler(name string, handler http.Handler) {
	a.Handlers[name] = handler
}

func (a *Application[I, O]) WithHealthChecker(healthChecker health.Checker) {
	a.HealthChecker = healthChecker
}

func (a *Application[I, O]) WithCloser(closer func() error) {
	a.Closers = append(a.Closers, closer)
}

func (a *Application[I, O]) WithStateSyncStarted(onStateSyncStarted ...func(context.Context) error) {
	a.OnStateSyncStarted = append(a.OnStateSyncStarted, onStateSyncStarted...)
}

func (a *Application[I, O]) WithNormalOpStarted(onNormalOpStartedF ...func(context.Context) error) {
	a.OnNormalOperationStarted = append(a.OnNormalOperationStarted, onNormalOpStartedF...)
}

// StartStateSync notifies the VM to enter DynamicStateSync mode.
// The caller is responsible to eventually call FinishStateSync with a fully populated
// last accepted state.
func (a *Application[I, O]) StartStateSync(ctx context.Context, block I) error {
	return a.vm.StartStateSync(ctx, block)
}

// FinishStateSync completes dynamic state sync mode and sets the last accepted block to
// the given input/output/accepted value.
func (a *Application[I, O]) FinishStateSync(ctx context.Context, input I, output O) error {
	return a.vm.FinishStateSync(ctx, input, output)
}

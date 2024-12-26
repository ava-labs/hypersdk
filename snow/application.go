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
	"github.com/ava-labs/hypersdk/lifecycle"
)

type Application[I Block, O Block, A Block] struct {
	vm *VM[I, O, A]

	Version         string
	Handlers        map[string]http.Handler
	HealthChecker   health.Checker
	Network         *p2p.Network
	StateSyncableVM block.StateSyncableVM
	Closers         []func() error

	Ready                    *lifecycle.AtomicBoolReady
	OnStateSyncStarted       []func(context.Context) error
	OnBootstrapStarted       []func(context.Context) error
	OnNormalOperationStarted []func(context.Context) error

	VerifiedSubs         []event.Subscription[O]
	RejectedSubs         []event.Subscription[O]
	AcceptedSubs         []event.Subscription[A]
	PreReadyAcceptedSubs []event.Subscription[I]
}

func (a *Application[I, O, A]) GetCovariantVM() *CovariantVM[I, O, A] {
	return a.vm.covariantVM
}

func (a *Application[I, O, A]) GetInputCovariantVM() *InputCovariantVM[I, O, A] {
	return &InputCovariantVM[I, O, A]{a.vm.covariantVM}
}

func (a *Application[I, O, A]) WithAcceptedSub(sub ...event.Subscription[A]) {
	a.AcceptedSubs = append(a.AcceptedSubs, sub...)
}

func (a *Application[I, O, A]) WithRejectedSub(sub ...event.Subscription[O]) {
	a.RejectedSubs = append(a.RejectedSubs, sub...)
}

func (a *Application[I, O, A]) WithVerifiedSub(sub ...event.Subscription[O]) {
	a.VerifiedSubs = append(a.VerifiedSubs, sub...)
}

func (a *Application[I, O, A]) WithPreReadyAcceptedSub(sub ...event.Subscription[I]) {
	a.PreReadyAcceptedSubs = append(a.PreReadyAcceptedSubs, sub...)
}

func (a *Application[I, O, A]) WithHandler(name string, handler http.Handler) {
	a.Handlers[name] = handler
}

func (a *Application[I, O, A]) WithHealthChecker(healthChecker health.Checker) {
	a.HealthChecker = healthChecker
}

func (a *Application[I, O, A]) WithCloser(closer func() error) {
	a.Closers = append(a.Closers, closer)
}

func (a *Application[I, O, A]) WithVersion(version string) {
	a.Version = version
}

func (a *Application[I, O, A]) WithStateSyncStarted(onStateSyncStarted ...func(context.Context) error) {
	a.OnStateSyncStarted = append(a.OnStateSyncStarted, onStateSyncStarted...)
}

func (a *Application[I, O, A]) WithNormalOpStarted(onNormalOpStartedF ...func(context.Context) error) {
	a.OnNormalOperationStarted = append(a.OnNormalOperationStarted, onNormalOpStartedF...)
}

// StartStateSync notifies the VM to enter DynamicStateSync mode.
// The caller is responsible to eventually call FinishStateSync with a fully populated
// last accepted state.
func (a *Application[I, O, A]) StartStateSync(ctx context.Context, block I) error {
	return a.vm.StartStateSync(ctx, block)
}

// FinishStateSync completes dynamic state sync mode and sets the last accepted block to
// the given input/ouput/accepted value.
func (a *Application[I, O, A]) FinishStateSync(ctx context.Context, input I, output O, accepted A) error {
	return a.vm.FinishStateSync(ctx, input, output, accepted)
}

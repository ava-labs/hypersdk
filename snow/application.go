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

type Application[I Block, O Block, A Block] struct {
	vm *VM[I, O, A]

	Version         string
	Handlers        map[string]http.Handler
	HealthChecker   health.Checker
	Network         *p2p.Network
	StateSyncableVM block.StateSyncableVM
	Closers         []namedCloser

	OnStateSyncStarted       []func(context.Context) error
	OnBootstrapStarted       []func(context.Context) error
	OnNormalOperationStarted []func(context.Context) error

	VerifiedSubs         []event.Subscription[O]
	RejectedSubs         []event.Subscription[O]
	AcceptedSubs         []event.Subscription[A]
	PreReadyAcceptedSubs []event.Subscription[I]
}

type namedCloser struct {
	name  string
	close func() error
}

// GetInputCovariantVM returns the VM implementation that returns the wrapper around the generic
// types (instead of the snowman.Block type)
func (a *Application[I, O, A]) GetInputCovariantVM() *InputCovariantVM[I, O, A] {
	return &InputCovariantVM[I, O, A]{a.vm}
}

func (a *Application[I, O, A]) AddAcceptedSub(sub ...event.Subscription[A]) {
	a.AcceptedSubs = append(a.AcceptedSubs, sub...)
}

func (a *Application[I, O, A]) AddRejectedSub(sub ...event.Subscription[O]) {
	a.RejectedSubs = append(a.RejectedSubs, sub...)
}

func (a *Application[I, O, A]) AddVerifiedSub(sub ...event.Subscription[O]) {
	a.VerifiedSubs = append(a.VerifiedSubs, sub...)
}

func (a *Application[I, O, A]) AddPreReadyAcceptedSub(sub ...event.Subscription[I]) {
	a.PreReadyAcceptedSubs = append(a.PreReadyAcceptedSubs, sub...)
}

func (a *Application[I, O, A]) AddHandler(name string, handler http.Handler) {
	a.Handlers[name] = handler
}

func (a *Application[I, O, A]) AddHealthCheck(healthChecker health.Checker) {
	a.HealthChecker = healthChecker
}

func (a *Application[I, O, A]) AddCloser(name string, closer func() error) {
	a.Closers = append(a.Closers, namedCloser{name, closer})
}

func (a *Application[I, O, A]) AddStateSyncStarter(onStateSyncStarted ...func(context.Context) error) {
	a.OnStateSyncStarted = append(a.OnStateSyncStarted, onStateSyncStarted...)
}

func (a *Application[I, O, A]) AddNormalOpStarter(onNormalOpStartedF ...func(context.Context) error) {
	a.OnNormalOperationStarted = append(a.OnNormalOperationStarted, onNormalOpStartedF...)
}

// StartStateSync notifies the VM to enter DynamicStateSync mode.
// The caller is responsible to eventually call FinishStateSync with a fully populated
// last accepted state.
func (a *Application[I, O, A]) StartStateSync(ctx context.Context, block I) error {
	return a.vm.startStateSync(ctx, block)
}

// FinishStateSync completes dynamic state sync mode and sets the last accepted block to
// the given input/output/accepted value.
func (a *Application[I, O, A]) FinishStateSync(ctx context.Context, input I, output O, accepted A) error {
	return a.vm.finishStateSync(ctx, input, output, accepted)
}

// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"errors"
	"net/http"

	"github.com/ava-labs/avalanchego/api/health"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/statesync"
)

type Options[I Block, O Block, A Block] struct {
	vm *VM[I, O, A]

	Version         string
	Handlers        map[string]http.Handler
	HealthChecker   health.Checker
	Network         *p2p.Network
	StateSyncClient *statesync.Client[*StatefulBlock[I, O, A]]
	StateSyncServer *statesync.Server[*StatefulBlock[I, O, A]]
	Closers         []func() error

	Ready *GroupReady

	OnBootstrapStarted       []func(context.Context) error
	OnNormalOperationStarted []func(context.Context) error

	VerifiedSubs         []event.Subscription[O]
	RejectedSubs         []event.Subscription[O]
	AcceptedSubs         []event.Subscription[A]
	PreReadyAcceptedSubs []event.Subscription[I]
}

func (o *Options[I, O, A]) WithAcceptedSub(sub ...event.Subscription[A]) {
	o.AcceptedSubs = append(o.AcceptedSubs, sub...)
}

func (o *Options[I, O, A]) WithRejectedSub(sub ...event.Subscription[O]) {
	o.RejectedSubs = append(o.RejectedSubs, sub...)
}

func (o *Options[I, O, A]) WithVerifiedSub(sub ...event.Subscription[O]) {
	o.VerifiedSubs = append(o.VerifiedSubs, sub...)
}

func (o *Options[I, O, A]) WithPreReadyAcceptedSub(sub ...event.Subscription[I]) {
	o.PreReadyAcceptedSubs = append(o.PreReadyAcceptedSubs, sub...)
}

func (o *Options[I, O, A]) WithHandler(name string, handler http.Handler) {
	o.Handlers[name] = handler
}

func (o *Options[I, O, A]) WithHealthChecker(healthChecker health.Checker) {
	o.HealthChecker = healthChecker
}

func (o *Options[I, O, A]) WithCloser(closer func() error) {
	o.Closers = append(o.Closers, closer)
}

func (o *Options[I, O, A]) WithVersion(version string) {
	o.Version = version
}

func (o *Options[I, O, A]) WithReady(r ...Ready) error {
	added := o.Ready.Add(r...)
	if !added {
		return errors.New("could not add ready")
	}
	return nil
}

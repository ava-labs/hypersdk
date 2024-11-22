// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package event

import (
	"context"
	"errors"
)

var (
	_ Subscription[struct{}]        = (*SubscriptionFunc[struct{}])(nil)
	_ SubscriptionFactory[struct{}] = (*SubscriptionFuncFactory[struct{}])(nil)
)

// SubscriptionFactory returns an instance of a concrete Subscription
type SubscriptionFactory[T any] interface {
	New() (Subscription[T], error)
}

// Subscription defines how to consume events
type Subscription[T any] interface {
	// Accept returns fatal errors
	Accept(ctx context.Context, t T) error
	// Close returns fatal errors
	Close() error
}

type SubscriptionFuncFactory[T any] struct {
	AcceptF func(ctx context.Context, t T) error
}

func (s SubscriptionFuncFactory[T]) New() (Subscription[T], error) {
	return SubscriptionFunc[T](s), nil
}

type SubscriptionFunc[T any] struct {
	AcceptF func(ctx context.Context, t T) error
}

func (s SubscriptionFunc[T]) Accept(ctx context.Context, t T) error {
	return s.AcceptF(ctx, t)
}

func (SubscriptionFunc[_]) Close() error {
	return nil
}

func NotifyAll[T any](ctx context.Context, e T, subs ...Subscription[T]) error {
	var errs []error
	for _, sub := range subs {
		if err := sub.Accept(ctx, e); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

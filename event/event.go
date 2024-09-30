// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package event

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
	Accept(t T) error
	// Close returns fatal errors
	Close() error
}

type SubscriptionFuncFactory[T any] struct {
	AcceptF func(t T) error
}

func (s SubscriptionFuncFactory[T]) New() (Subscription[T], error) {
	return SubscriptionFunc[T](s), nil
}

type SubscriptionFunc[T any] struct {
	AcceptF func(t T) error
}

func (s SubscriptionFunc[T]) Accept(t T) error {
	return s.AcceptF(t)
}

func (SubscriptionFunc[_]) Close() error {
	return nil
}

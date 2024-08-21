// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package event

// SubscriptionFactory returns an instance of a concrete Subscription
type SubscriptionFactory[T any] interface {
	New(chainDataDir string) (Subscription[T], error)
}

// Subscription defines how to consume events
type Subscription[T any] interface {
	// Accept returns fatal errors
	Accept(t T) error
	// Close returns fatal errors
	Close() error
}

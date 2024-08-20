// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package event

type SubscriptionFactory[T any] interface {
	New() (Subscription[T], error)
}

type Subscription[T any] interface {
	Accept(t T) error
	Close() error
}

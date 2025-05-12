// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

type Agent[T comparable] struct {
	Issuer   Issuer[T]
	Listener Listener[T]
}

func NewAgent[T comparable](
	issuer Issuer[T],
	listener Listener[T],
) Agent[T] {
	return Agent[T]{
		Issuer:   issuer,
		Listener: listener,
	}
}

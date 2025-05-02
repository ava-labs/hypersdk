// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

type Agent[T, U comparable] struct {
	Generator TxGenerator[T]
	Issuer    Issuer[T]
	Listener  Listener[T]
	Tracker   Tracker[U]
}

func NewAgent[T, U comparable](
	generator TxGenerator[T],
	issuer Issuer[T],
	listener Listener[T],
	tracker Tracker[U],
) Agent[T, U] {
	return Agent[T, U]{
		Generator: generator,
		Issuer:    issuer,
		Listener:  listener,
		Tracker:   tracker,
	}
}

func GetTotalObservedConfirmed[T, U comparable](agents []Agent[T, U]) uint64 {
	total := uint64(0)
	for _, agent := range agents {
		total += agent.Tracker.GetObservedConfirmed()
	}
	return total
}

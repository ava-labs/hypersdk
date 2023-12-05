// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

type Meter interface {
	// GetBalance returns the balance of the meter's units remaining.
	GetBalance() uint64
	// Spend attempts to spend the given amount of units. If the meter does not have sufficient units it will return an error and the balance will become zero.
	Spend(uint64) (uint64, error)
	// AddUnits add units back to the meters and returns the new balance.
	AddUnits(uint64) (uint64, error)
	// TransferUnitsTo transfers units from this meter to the given meter, returns
	// the new balance.
	TransferUnitsTo(to Meter, units uint64) (uint64, error)
}

type Instance interface {
	GetFunc(name string) (*Func, error)
	GetExport(name string) (*Export, error)
	Memory() (*Memory, error)
}

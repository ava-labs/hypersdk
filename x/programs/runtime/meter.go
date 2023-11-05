// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"github.com/bytecodealliance/wasmtime-go/v14"
)

const NoUnits = 0

var _ Meter = (*meter)(nil)

// NewMeter returns a new meter.
func NewMeter(store *wasmtime.Store) Meter {
	return &meter{
		store: store,
	}
}

type meter struct {
	maxUnits uint64
	store    *wasmtime.Store
}

func (m *meter) GetBalance() uint64 {
	consumed, ok := m.store.FuelConsumed()
	if !ok {
		return 0
	}
	if m.maxUnits < consumed {
		panic("meter balance should never be negative")
	}

	return m.maxUnits - consumed
}

func (m *meter) Spend(units uint64) (uint64, error) {
	if m.GetBalance() < units {
		return 0, ErrInsufficientUnits
	}
	return m.store.ConsumeFuel(units)
}

func (m *meter) AddUnits(units uint64) (uint64, error) {
	err := m.store.AddFuel(units)
	if err != nil {
		return 0, err
	}
	m.maxUnits += units

	return m.GetBalance(), nil
}

// TransferUnitsTo moves units from this meter to another meter.
func (m *meter) TransferUnitsTo(to Meter, units uint64) (uint64, error) {
	// TODO: add rollback support?

	// spend units from this meter
	_, err := m.Spend(units)
	if err != nil {
		return 0, err
	}
	// add units to the other meter
	return to.AddUnits(units)
}

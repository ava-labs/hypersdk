// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"errors"
	"fmt"

	"github.com/ava-labs/hypersdk/x/programs/engine"
)

const NoUnits = 0

var ErrInsufficientUnits = errors.New("insufficient units")

var _ Meter = (*meter)(nil)

// NewMeter returns a new meter initialized with max units.
func NewMeter(store *engine.Store, maxUnits uint64) (Meter, error) {
	m := &meter{
		store: store,
	}
	_, err := m.AddUnits(maxUnits)
	if err != nil {
		return nil, err
	}
	return m, nil
}

type meter struct {
	maxUnits uint64
	store    *engine.Store
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
		// spend all remaining units
		_, err := m.Spend(m.GetBalance())
		if err != nil {
			return 0, fmt.Errorf("%w: %w", ErrInsufficientUnits, err)
		}
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
	// spend units from this meter
	_, err := m.Spend(units)
	if err != nil {
		return 0, err
	}
	// add units to the other meter
	_, err = to.AddUnits(units)
	if err != nil {
		return 0, err
	}
	return m.GetBalance(), nil
}

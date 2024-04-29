// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"errors"
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

const (
	NoUnits = uint64(0)

	defaultLimitMaxTableElements = 4096
	defaultLimitMaxTables        = 1
	defaultLimitMaxInstances     = 32
	defaultLimitMaxMemories      = 1
)

var (
	ErrInsufficientUnits = errors.New("insufficient units")
	ErrMeteredStore      = errors.New("store is already metered")
)

// NewStore creates a new engine store.
func NewStore(e *Engine, cfg *StoreConfig) *Store {
	wasmStore := wasmtime.NewStore(e.wasmEngine)
	wasmStore.Limiter(
		int64(cfg.limitMaxMemory),
		cfg.limitMaxTableElements,
		cfg.limitMaxInstances,
		cfg.limitMaxTables,
		cfg.limitMaxMemories,
	)
	// set initial epoch deadline to 1. This ensures that any stop calls to the
	// engine will affect this store.
	wasmStore.SetEpochDeadline(1)

	return &Store{wasmStore: wasmStore}
}

// Store is a wrapper around a wasmtime.Store.
type Store struct {
	wasmStore *wasmtime.Store
	// metered indicates whether this store is metered or not.
	metered bool
	// maxUnits is the maximum number of units that can be consumed by this store.
	maxUnits uint64
}

// SetEpochDeadline will configure the relative deadline, from the current
// engine's epoch number, after which wasm code will be interrupted.
func (s *Store) SetEpochDeadline(epochDeadline uint64) {
	s.wasmStore.SetEpochDeadline(epochDeadline)
}

// GetEngine returns the underlying wasmtime engine associated with this store.
func (s *Store) GetEngine() *wasmtime.Engine {
	return s.wasmStore.Engine
}

// UnitsConsumed returns the amount of fuel consumed by this store.
func (s *Store) UnitsConsumed() (uint64, bool) {
	return s.wasmStore.FuelConsumed()
}

// ConsumeUnits will consume the provided units.
func (s *Store) ConsumeUnits(units uint64) (uint64, error) {
	return s.wasmStore.ConsumeFuel(units)
}

// GetMaxUnits returns the maximum number of units that can be consumed by this store.
func (s *Store) GetMaxUnits() uint64 {
	return s.maxUnits
}

// GetBalanceUnits returns the balance of the maximum number of units that can
// be consumed by this store and the number of units consumed.
func (s *Store) GetBalanceUnits() (uint64, error) {
	consumed, ok := s.UnitsConsumed()
	if !ok {
		return 0, errors.New("failed to get units consumed: metering not enabled")
	}
	return (s.maxUnits - consumed), nil
}

// AddUnits will add the provided units to the meter.
func (s *Store) AddUnits(units uint64) error {
	s.maxUnits += units
	return s.wasmStore.AddFuel(units)
}

// SetWasi will configure the Wasi configuration for this store.
func (s *Store) SetWasi(cfg *wasmtime.WasiConfig) {
	s.wasmStore.SetWasi(cfg)
}

// Get returns the underlying wasmtime store.
func (s *Store) Get() *wasmtime.Store {
	return s.wasmStore
}

// NewStoreConfig returns a new store config.
func NewStoreConfig() *StoreConfig {
	return &StoreConfig{
		limitMaxMemory:        DefaultLimitMaxMemory,
		limitMaxTableElements: defaultLimitMaxTableElements,
		limitMaxTables:        defaultLimitMaxTables,
		limitMaxInstances:     defaultLimitMaxInstances,
		limitMaxMemories:      defaultLimitMaxMemories,
	}
}

// StoreConfig is the configuration for an engine store.
type StoreConfig struct {
	limitMaxMemory        uint32
	limitMaxTableElements int64
	limitMaxTables        int64
	limitMaxInstances     int64
	limitMaxMemories      int64
}

// SetLimitMaxMemory sets the limit max memory.
func (c *StoreConfig) SetLimitMaxMemory(limitMaxMemory uint32) *StoreConfig {
	c.limitMaxMemory = limitMaxMemory
	return c
}

// NewMeter returns a new meter. A store can only be registered to a single
// meter.
func NewMeter(store *Store) (*Meter, error) {
	if store.metered {
		return nil, ErrMeteredStore
	}
	store.metered = true
	return &Meter{
		store: store,
	}, nil
}

// Meter is an abstraction of a store.
type Meter struct {
	store *Store
}

// GetBalance returns the balance in units of the meter.
func (m *Meter) GetBalance() (uint64, error) {
	return m.store.GetBalanceUnits()
}

// Spend will spend the provided units from the meter. If the balance is less
// than the provided units, the balance will be spent.
func (m *Meter) Spend(units uint64) (uint64, error) {
	balance, err := m.GetBalance()
	if err != nil {
		return 0, err
	}
	if balance < units {
		_, err := m.store.ConsumeUnits(balance)
		if err != nil {
			return 0, fmt.Errorf("%w: %w", ErrInsufficientUnits, err)
		}
		return 0, ErrInsufficientUnits
	}
	return m.store.ConsumeUnits(units)
}

func (m *Meter) addUnits(units uint64) error {
	return m.store.AddUnits(units)
}

// TransferUnitsTo moves units from this meter to another meter.
func (m *Meter) TransferUnitsTo(to *Meter, units uint64) (uint64, error) {
	// spend units from this meter
	_, err := m.Spend(units)
	if err != nil {
		return 0, err
	}
	// add units to the other meter
	err = to.addUnits(units)
	if err != nil {
		return 0, err
	}
	return m.GetBalance()
}

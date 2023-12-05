// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"github.com/bytecodealliance/wasmtime-go/v14"
)

const (
	defaultLimitMaxTableElements = 4096
	defaultLimitMaxTables        = 1
	defaultLimitMaxInstances     = 32
	defaultLimitMaxMemories      = 1
)

func NewStoreConfig() *StoreConfig {
	return &StoreConfig{
		limitMaxMemory:        DefaultLimitMaxMemory,
		limitMaxTableElements: defaultLimitMaxTableElements,
		limitMaxTables:        defaultLimitMaxTables,
		limitMaxInstances:     defaultLimitMaxInstances,
		limitMaxMemories:      defaultLimitMaxMemories,
	}
}

func (c *StoreConfig) SetLimitMaxMemory(limitMaxMemory int64) {
	c.limitMaxMemory = limitMaxMemory
}

type StoreConfig struct {
	limitMaxMemory        int64
	limitMaxTableElements int64
	limitMaxTables        int64
	limitMaxInstances     int64
	limitMaxMemories      int64
}

func NewStore(e *Engine, cfg *StoreConfig) *Store {
	inner := wasmtime.NewStore(e.inner)
	inner.Limiter(
		cfg.limitMaxMemory,
		cfg.limitMaxTableElements,
		cfg.limitMaxInstances,
		cfg.limitMaxTables,
		cfg.limitMaxMemories,
	)
	return &Store{inner: inner}
}

type Store struct {
	inner *wasmtime.Store
}

func (s *Store) SetEpochDeadline(epochDeadline uint64) {
	s.inner.SetEpochDeadline(epochDeadline)
}

func (s *Store) Engine() *wasmtime.Engine {
	return s.inner.Engine
}

func (s *Store) FuelConsumed() (uint64, bool) {
	return s.inner.FuelConsumed()
}

func (s *Store) ConsumeFuel(units uint64) (uint64, error) {
	return s.inner.ConsumeFuel(units)
}

func (s *Store) AddFuel(units uint64) error {
	return s.inner.AddFuel(units)
}

func (s *Store) SetWasi(cfg *wasmtime.WasiConfig) {
	s.inner.SetWasi(cfg)
}

func (s *Store) Inner() *wasmtime.Store {
	return s.inner
}

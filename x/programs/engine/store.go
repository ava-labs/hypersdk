// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"github.com/bytecodealliance/wasmtime-go/v14"
)

func NewStore(e *Engine) (*Store, error) {
	inner := wasmtime.NewStore(e.inner)
	inner.Limiter(
		e.cfg.limitMaxMemory,
		e.cfg.limitMaxTableElements,
		e.cfg.limitMaxInstances,
		e.cfg.limitMaxTables,
		e.cfg.limitMaxMemories,
	)

	return &Store{inner: inner}, nil
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

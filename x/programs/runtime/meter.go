// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"

	"github.com/tetratelabs/wazero/api"

	"github.com/ava-labs/avalanchego/utils/logging"
)

var (
	ErrMeterInsufficientBalance       = errors.New("operation failed insufficient balance")
	_                           Meter = (*meter)(nil)
)

type Meter interface {
	api.Meter
	GetBalance(context.Context) uint64
}

// NewMeter returns a meter capable of tracking the cost of operations.
func NewMeter(log logging.Logger, maxFee uint64, costMap map[string]uint64) *meter {
	return &meter{
		log:     log,
		costMap: costMap,
		balance: maxFee,
	}
}

type meter struct {
	lock         sync.RWMutex
	costMap      map[string]uint64
	balance      uint64
	initialError error

	log logging.Logger
}

func (m *meter) AddCost(_ context.Context, op string) error {
	if err := m.closed(); err != nil {
		return err
	}
	m.lock.Lock()
	defer m.lock.Unlock()

	fee := m.costMap[op]
	if fee > m.balance {
		m.log.Debug("insufficient balance", zap.Uint64("fee", fee), zap.Uint64("balance", m.balance))
		m.initialError = ErrMeterInsufficientBalance
		return m.initialError
	}

	m.balance -= fee

	return nil
}

func (m *meter) GetBalance(_ context.Context) uint64 {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.balance
}

func (m *meter) closed() error {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.initialError
}

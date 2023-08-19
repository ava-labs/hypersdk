// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero/api"
)

var (
	_                      Meter = (*meter)(nil)
	ErrMeterInvalidBalance       = errors.New("operation cost greater than balance")
)

type Meter interface {
	api.Meter
	Run(context.Context)
}

func NewMeter(maxFee uint64, costMap map[string]uint64) *meter {
	return &meter{
		costMap: costMap,
		balance: maxFee,
		waitCh:  make(chan struct{}),
	}
}

type meter struct {
	lock    sync.RWMutex
	costMap map[string]uint64
	balance uint64

	waitCh chan struct{}
}

func (m *meter) AddCost(_ context.Context, op string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	cost := m.costMap[op]
	if cost > m.balance {
		defer close(m.waitCh)
		m.waitCh <- struct{}{}
		return fmt.Errorf("%w: %d: %d", ErrMeterInvalidBalance, cost, m.balance)
	}

	m.balance -= cost
	return nil
}

func (m *meter) GetBalance() uint64 {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.balance
}

func (m *meter) Run(ctx context.Context) {
	select {
	case <-m.waitCh:
	case <-ctx.Done():
	}
}

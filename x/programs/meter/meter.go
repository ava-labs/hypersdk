// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package meter

import (
	"context"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero/api"
)

var _ Meter = (*meter)(nil)

type Meter interface {
	api.Meter
	Run(context.Context) error
}

func New(maxFee uint64, costMap map[string]uint64) *meter {
	return &meter{
		costMap: costMap,
		balance: maxFee,
		errCh:   make(chan error, 1),
	}
}

type meter struct {
	lock    sync.RWMutex
	costMap map[string]uint64
	balance uint64
	closed  bool

	errCh chan error
}

func (m *meter) AddCost(op string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.closed {
		return
	}

	cost := m.costMap[op]
	if cost > m.balance {
		m.stop()
		return
	}

	m.balance -= cost
}

func (m *meter) GetCost() uint64 {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.balance
}

func (m *meter) Run(ctx context.Context) error {
	select {
	case err := <-m.errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

func (m *meter) stop() {
	m.closed = true
	m.errCh <- fmt.Errorf("operation cost greater than balance: %d", m.balance)
	close(m.errCh)
}

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
	return &meter{costMap: costMap, balance: maxFee}
}

type meter struct {
	lock    sync.Mutex
	costMap map[string]uint64
	balance uint64

	ErrCh chan error
}

func (m *meter) AddCost(op string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	cost := m.costMap[op]
	if cost > m.balance {
		m.ErrCh <- fmt.Errorf("operation cost: %d greater than balance: %d", cost, m.balance)
		close(m.ErrCh)
		return
	}

	m.balance -= cost
}

func (m *meter) GetCost() uint64 {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.balance
}

func (m *meter) Run(ctx context.Context) error {
	select {
	case err := <-m.ErrCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

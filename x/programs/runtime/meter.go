package runtime

import "github.com/bytecodealliance/wasmtime-go/v12"

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
	return m.store.ConsumeFuel(units)
}

func (m *meter) AddUnits(units uint64) error {
	err := m.store.AddFuel(units)
	if err != nil {
		return err
	}
	m.maxUnits += units

	return nil
}

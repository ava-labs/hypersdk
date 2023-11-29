package engine

type Meter interface {
	// GetBalance returns the balance of the meter's units remaining.
	GetBalance() uint64
	// Spend attempts to spend the given amount of units. If the meter has
	Spend(uint64) (uint64, error)
	// AddUnits add units back to the meters and returns the new balance.
	AddUnits(uint64) (uint64, error)
	// TransferUnitsTo transfers units from this meter to the given meter, returns
	// the new balance of this meter.
	TransferUnitsTo(to Meter, units uint64) (uint64, error)
}

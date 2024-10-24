package runtime

import "github.com/ava-labs/hypersdk/state"

// default implementation of the ContractManager interface
type ContractStateManager struct {
	// this state mutable should have its own prefix that doesn't conflict with other state
	db state.Mutable
}

func NewContractStateManager(
	db state.Mutable,
) *ContractStateManager {
	return &ContractStateManager{
		db: db,
	}
}


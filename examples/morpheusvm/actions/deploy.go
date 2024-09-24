package actions

import (
	"context"
	"crypto/sha256"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*Deploy)(nil)

// Deploy, deploys a contract to the chain with the given bytes
type Deploy struct {
	// ContractBytes is the wasm bytes of the contract being deployed
	ContractBytes []byte
}

// Action Interface

// units to execute this action
func (*Deploy) ComputeUnits(chain.Rules) uint64 {
	// TODO: charge more if contract has not been deployed before(because adding to storage should be more expensive)
	return consts.DeployUnits
}

// Why is StateKeysMaxChunks part of the action interface?
func (*Deploy) StateKeysMaxChunks() []uint16 {
	return nil
}

// Specify all statekeys 
func (*Deploy) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	return nil
}

// Execute deploys the contract to the chain, returning {deploy contract ID, deployed contract address}
func (d *Deploy) Execute(ctx context.Context, rules chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (codec.Typed, error) {
	// gets the contract ID by hashing the contract bytes
	contractID := sha256.Sum256(d.ContractBytes)
	
	// checks if the contract is already deployed
		// if _, err := mu.Get(ctx, contractID); err == nil {
	// add the contract bytes to the storage
	// create a new account associated with the contract


	return nil, nil
}

// Object interface
func (*Deploy) GetTypeID() uint8 {
	return consts.DeployID
}

func (*Deploy) ValidRange(rules chain.Rules) (int64, int64) {
	return -1, -1
}
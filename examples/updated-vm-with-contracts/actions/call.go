// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/consts"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

var _ chain.Action = (*Call)(nil)

type StateKeyPermission struct {
	Key        string
	Permission state.Permissions
}

// Deploy, deploys a contract to the chain with the given bytes
type Call struct {
	// address of the contract
	ContractAddress codec.Address `json:"contractAddress"`

	// value passed into the contract
	Value uint64 `json:"value"`

	// function name to call
	FunctionName string `json:"functionName"`

	// borsh serialized arguments
	Args []byte `json:"args"`

	// amount of gas to use
	Fuel uint64 `json:"fuel"`

	// state keys that will be accessed during the execution
	SpecifiedStateKeys []StateKeyPermission `json:"statekeys"`

	// runtime
	r *runtime.WasmRuntime
}

// units to execute this action
func (*Call) ComputeUnits(chain.Rules) uint64 {
	return consts.CallUnits
}

func (t *Call) StateKeys(_ codec.Address) state.Keys {
	result := state.Keys{}
	for _, stateKeyPermission := range t.SpecifiedStateKeys {
		result.Add(stateKeyPermission.Key, stateKeyPermission.Permission)
	}
	return result
}

// Execute deploys the contract to the chain, returning {deploy contract ID, deployed contract address}
func (c *Call) Execute(ctx context.Context, rules chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (codec.Typed, error) {
	utils.Outf("{{green}} GREEN FN:{{/}} %s\n", "INSIDE ACTION")
	if c.r == nil {
		config := runtime.NewConfig()
		config.SetDebugInfo(true)
		rt := runtime.NewRuntime(config, logging.NewLogger("test"))
		c.r = rt
	}
	// create runtime call info
	contractState := &storage.ContractStateManager{Mutable: mu}

	runtimeCallInfo := &runtime.CallInfo{
		State:        contractState,
		Actor:        actor,
		FunctionName: c.FunctionName,
		Contract:     c.ContractAddress,
		Params:       c.Args,
		Fuel:         c.Fuel,
		// pass in the timestamp as the height
		Height:    uint64(timestamp),
		Timestamp: uint64(timestamp),
		ActionID:  actionID,
	}

	result, err := c.r.CallContract(ctx, runtimeCallInfo)
	if err != nil {
		return nil, err
	}

	return &CallOutput{result: result}, nil
}

// Object interface
func (*Call) GetTypeID() uint8 {
	return consts.CallId
}

func (*Call) ValidRange(rules chain.Rules) (int64, int64) {
	return -1, -1
}

type CallOutput struct {
	result []byte
}

func (*CallOutput) GetTypeID() uint8 {
	return consts.CallOutputId
}

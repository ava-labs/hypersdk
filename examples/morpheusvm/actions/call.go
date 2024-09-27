package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

var _ chain.Action = (*Call)(nil)

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

	// runtime
	r *runtime.WasmRuntime
}

// units to execute this action
func (*Call) ComputeUnits(chain.Rules) uint64 {
	return consts.CallUnits
}

func (c *Call) StateKeysMaxChunks() []uint16 {
	return []uint16{}
}

// Specify all statekeys Execute can touch
func (c *Call) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	return state.Keys{}
}

// Execute deploys the contract to the chain, returning {deploy contract ID, deployed contract address}
func (c *Call) Execute(ctx context.Context, rules chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (codec.Typed, error) {
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

var _ chain.Marshaler = (*Call)(nil)

func (c *Call) Size() int {
	// TODO: don't hardcode uint sizes
	return codec.AddressLen + 8 + len(c.FunctionName) + len(c.Args) + 8
}

func (c *Call) Marshal(p *codec.Packer) {
	p.PackBytes(c.ContractAddress[:])
	p.PackUint64(c.Value)
	p.PackString(c.FunctionName)
	p.PackBytes(c.Args)
	p.PackUint64(c.Fuel)

}

func UnmarshalCall(p *codec.Packer) (chain.Action, error) {
	var callContract Call
	contractAddress := p.Packer.UnpackBytes()
	value := p.UnpackUint64(true)
	functionName := p.UnpackString(true)
	args := p.Packer.UnpackBytes()
	fuel := p.UnpackUint64(true)

	callContract.ContractAddress = codec.Address(contractAddress)
	callContract.Value = value
	callContract.FunctionName = functionName
	callContract.Args = args
	callContract.Fuel = fuel

	return &callContract, nil
}

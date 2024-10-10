// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/hex"
	"net/http"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/actions"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/consts"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

const JSONRPCEndpoint = "/updatedvmwithcontractsapi"

var _ api.HandlerFactory[api.VM] = (*jsonRPCServerFactory)(nil)

type jsonRPCServerFactory struct{}

func (jsonRPCServerFactory) New(vm api.VM) (api.Handler, error) {
	handler, err := api.NewJSONRPCHandler(consts.Name, NewJSONRPCServer(vm))
	return api.Handler{
		Path:    JSONRPCEndpoint,
		Handler: handler,
	}, err
}

type JSONRPCServer struct {
	vm api.VM
}

func NewJSONRPCServer(vm api.VM) *JSONRPCServer {
	return &JSONRPCServer{vm: vm}
}

type GenesisReply struct {
	Genesis *genesis.DefaultGenesis `json:"genesis"`
}

func (j *JSONRPCServer) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
	reply.Genesis = j.vm.Genesis().(*genesis.DefaultGenesis)
	return nil
}

type BalanceArgs struct {
	Address codec.Address `json:"address"`
}

type BalanceReply struct {
	Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) Balance(req *http.Request, args *BalanceArgs, reply *BalanceReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Balance")
	defer span.End()

	balance, err := storage.GetBalanceFromState(ctx, j.vm.ReadState, args.Address)
	if err != nil {
		return err
	}
	reply.Amount = balance
	return err
}


// Contract endpoints
type SimulateCallTxArgs struct {
	CallTx actions.Call  `json:"callTx"`
	Actor  codec.Address `json:"actor"`
}

type SimulateStateKey struct {
	HexKey      string `json:"hex"`
	Permissions byte   `json:"perm"`
}
type SimulateCallTxReply struct {
	StateKeys    []SimulateStateKey `json:"stateKeys"`
	FuelConsumed uint64             `json:"fuel"`
}

func (j *JSONRPCServer) SimulateCallContractTx(req *http.Request, args *SimulateCallTxArgs, reply *SimulateCallTxReply) (err error) {
	stateKeys, fuelConsumed, err := j.simulate(req.Context(), args.CallTx, args.Actor)
	if err != nil {
		return err
	}
	reply.StateKeys = make([]SimulateStateKey, 0, len(stateKeys))
	for key, permission := range stateKeys {
		reply.StateKeys = append(reply.StateKeys, SimulateStateKey{HexKey: hex.EncodeToString([]byte(key)), Permissions: byte(permission)})
	}
	reply.FuelConsumed = fuelConsumed
	return nil
}

func (j *JSONRPCServer) simulate(ctx context.Context, callAction actions.Call, actor codec.Address) (state.Keys, uint64, error) {
	currentState, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return nil, 0, err
	}
	recorder := storage.NewRecorder(currentState)
	callInfo := &runtime.CallInfo{
		Contract:     callAction.ContractAddress,
		Actor:        actor,
		State:        &storage.ContractStateManager{Mutable: recorder},
		FunctionName: callAction.FunctionName,
		Params:       callAction.Args,
		Fuel:         callAction.Fuel,
		Value:        callAction.Value,
	}
	_, err = wasmRuntime.CallContract(ctx, callInfo)
	return recorder.GetStateKeys(), callAction.Fuel - callInfo.RemainingFuel(), err
}

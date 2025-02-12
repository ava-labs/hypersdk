// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/evm/consts"
	"github.com/ava-labs/hypersdk/examples/evm/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

const JSONRPCEndpoint = "/evmapi"

var _ api.HandlerFactory[api.VM] = (*jsonRPCServerFactory)(nil)

var (
	errSimulateZeroActions   = errors.New("simulateAction expects at least a single action, none found")
	errTransactionExtraBytes = errors.New("transaction has extra bytes")
)

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

	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}

	balance, err := storage.GetBalance(ctx, im, storage.ToEVMAddress(args.Address))
	if err != nil {
		return err
	}
	reply.Amount = balance
	return err
}

type SimulatActionsArgs struct {
	Actions []codec.Bytes `json:"actions"`
	Actor   codec.Address `json:"actor"`
}

type SimulateActionResult struct {
	Output    codec.Bytes `json:"output"`
	StateKeys state.Keys  `json:"stateKeys"`
}

type SimulateActionsReply struct {
	ActionResults []SimulateActionResult `json:"actionresults"`
}

func (j *JSONRPCServer) SimulateActions(
	req *http.Request,
	args *SimulatActionsArgs,
	reply *SimulateActionsReply,
) error {
	j.vm.Logger().Info("SimulateActions")
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.SimulateActions")
	defer span.End()

	actionRegistry := j.vm.ActionCodec()
	var actions chain.Actions
	for _, actionBytes := range args.Actions {
		actionsReader := codec.NewReader(actionBytes, len(actionBytes))
		action, err := (*actionRegistry).Unmarshal(actionsReader)
		if err != nil {
			return err
		}
		if !actionsReader.Empty() {
			return errTransactionExtraBytes
		}
		actions = append(actions, action)
	}
	if len(actions) == 0 {
		return errSimulateZeroActions
	}
	currentState, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}

	ts := tstate.New(0)
	scope := state.SimulatedKeys{}
	tsv := ts.NewView(
		scope,
		currentState,
		0,
	)

	currentTime := time.Now().UnixMilli()
	blockCtx := chain.NewBlockContext(0, currentTime)
	for _, action := range actions {
		actionOutput, err := action.Execute(
			ctx,
			blockCtx,
			j.vm.Rules(currentTime),
			tsv,
			args.Actor,
			ids.Empty,
		)

		var actionResult SimulateActionResult
		if actionOutput == nil {
			actionResult.Output = []byte{}
		} else {
			actionResult.Output, err = chain.MarshalTyped(actionOutput)
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}
		}
		if err != nil {
			return err
		}
		actionResult.StateKeys = maps.Clone(scope.StateKeys())
		reply.ActionResults = append(reply.ActionResults, actionResult)
		// Reset state keys for the next action
		clear(scope)
	}
	return nil
}

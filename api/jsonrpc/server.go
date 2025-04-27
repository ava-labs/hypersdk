// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package jsonrpc

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

const (
	Endpoint = "/coreapi"
)

var errNoActionsToExecute = errors.New("no actions to execute")

var (
	_ api.HandlerFactory[api.VM] = (*JSONRPCServerFactory)(nil)

	errSimulateZeroActions = errors.New("simulateAction expects at least a single action, none found")
)

type JSONRPCServerFactory struct{}

func (JSONRPCServerFactory) New(vm api.VM) (api.Handler, error) {
	handler, err := api.NewJSONRPCHandler(api.Name, NewJSONRPCServer(vm))
	if err != nil {
		return api.Handler{}, err
	}

	return api.Handler{
		Path:    Endpoint,
		Handler: handler,
	}, nil
}

type JSONRPCServer struct {
	vm api.VM
}

func NewJSONRPCServer(vm api.VM) *JSONRPCServer {
	return &JSONRPCServer{vm}
}

type PingReply struct {
	Success bool `json:"success"`
}

func (j *JSONRPCServer) Ping(_ *http.Request, _ *struct{}, reply *PingReply) (err error) {
	j.vm.Logger().Info("ping")
	reply.Success = true
	return nil
}

type NetworkReply struct {
	NetworkID uint32 `json:"networkId"`
	SubnetID  ids.ID `json:"subnetId"`
	ChainID   ids.ID `json:"chainId"`
}

func (j *JSONRPCServer) Network(_ *http.Request, _ *struct{}, reply *NetworkReply) (err error) {
	reply.NetworkID = j.vm.NetworkID()
	reply.SubnetID = j.vm.SubnetID()
	reply.ChainID = j.vm.ChainID()
	return nil
}

type SubmitTxArgs struct {
	Tx []byte `json:"tx"`
}

type SubmitTxReply struct {
	TxID ids.ID `json:"txId"`
}

func (j *JSONRPCServer) SubmitTx(
	req *http.Request,
	args *SubmitTxArgs,
	reply *SubmitTxReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.SubmitTx")
	defer span.End()

	tx, err := chain.UnmarshalTx(args.Tx, j.vm.GetParser())
	if err != nil {
		return fmt.Errorf("%w: unable to unmarshal on public service", err)
	}
	txID := tx.GetID()
	reply.TxID = txID
	return j.vm.Submit(ctx, []*chain.Transaction{tx})[0]
}

type LastAcceptedReply struct {
	Height    uint64 `json:"height"`
	BlockID   ids.ID `json:"blockId"`
	Timestamp int64  `json:"timestamp"`
}

func (j *JSONRPCServer) LastAccepted(req *http.Request, _ *struct{}, reply *LastAcceptedReply) error {
	blk, err := j.vm.LastAcceptedBlock(req.Context())
	if err != nil {
		return err
	}
	reply.Height = blk.Hght
	reply.BlockID = blk.GetID()
	reply.Timestamp = blk.Tmstmp
	return nil
}

type UnitPricesReply struct {
	UnitPrices fees.Dimensions `json:"unitPrices"`
}

func (j *JSONRPCServer) UnitPrices(
	req *http.Request,
	_ *struct{},
	reply *UnitPricesReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.UnitPrices")
	defer span.End()

	unitPrices, err := j.vm.UnitPrices(ctx)
	if err != nil {
		return err
	}
	reply.UnitPrices = unitPrices
	return nil
}

type GetABIArgs struct{}

type GetABIReply struct {
	ABI abi.ABI `json:"abi"`
}

func (j *JSONRPCServer) GetABI(_ *http.Request, _ *GetABIArgs, reply *GetABIReply) error {
	reply.ABI = j.vm.GetABI()
	return nil
}

type ExecuteActionArgs struct {
	Actor   codec.Address `json:"actor"`
	Actions [][]byte      `json:"actions"`
}

type ExecuteActionReply struct {
	Outputs [][]byte `json:"outputs"`
	Error   string   `json:"error"`
}

func (j *JSONRPCServer) ExecuteActions(
	req *http.Request,
	args *ExecuteActionArgs,
	reply *ExecuteActionReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.ExecuteAction")
	defer span.End()

	if len(args.Actions) == 0 {
		return errNoActionsToExecute
	}
	ruleFactory := j.vm.GetRuleFactory()
	now := time.Now().UnixMilli()
	rules := ruleFactory.GetRules(now)
	if maxActionsPerTx := int(rules.GetMaxActionsPerTx()); len(args.Actions) > maxActionsPerTx {
		return fmt.Errorf("exceeded max actions per simulation: %d", maxActionsPerTx)
	}
	actions := make([]chain.Action, 0, len(args.Actions))
	txParser := j.vm.GetParser()
	for _, actionBytes := range args.Actions {
		action, err := txParser.ParseAction(actionBytes)
		if err != nil {
			return fmt.Errorf("failed to unmashal action: %w", err)
		}
		actions = append(actions, action)
	}

	storage := make(map[string][]byte)
	ts := tstate.New(1)

	for actionIndex, action := range actions {
		// Get expected state keys
		stateKeysWithPermissions := action.StateKeys(args.Actor, chain.CreateActionID(ids.Empty, uint8(actionIndex)))

		// flatten the map to a slice of keys
		storageKeysToRead := make([][]byte, 0, len(stateKeysWithPermissions))
		for key := range stateKeysWithPermissions {
			storageKeysToRead = append(storageKeysToRead, []byte(key))
		}

		values, errs := j.vm.ReadState(ctx, storageKeysToRead)
		for _, err := range errs {
			if err != nil && !errors.Is(err, database.ErrNotFound) {
				return fmt.Errorf("failed to read state: %w", err)
			}
		}
		for i, value := range values {
			if value == nil {
				continue
			}
			storage[string(storageKeysToRead[i])] = value
		}

		tsv := ts.NewView(
			stateKeysWithPermissions,
			state.ImmutableStorage(storage),
			len(stateKeysWithPermissions),
		)

		output, err := action.Execute(
			ctx,
			rules,
			tsv,
			now,
			args.Actor,
			chain.CreateActionID(ids.Empty, uint8(actionIndex)),
		)
		if err != nil {
			reply.Error = fmt.Sprintf("failed to execute action: %s", err)
			return nil
		}
		tsv.Commit()

		reply.Outputs = append(reply.Outputs, output)
	}
	return nil
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
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.SimulateActions")
	defer span.End()

	txParser := j.vm.GetParser()
	actions := make([]chain.Action, 0, len(args.Actions))
	for _, actionBytes := range args.Actions {
		action, err := txParser.ParseAction(actionBytes)
		if err != nil {
			return err
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
	ruleFactory := j.vm.GetRuleFactory()
	rules := ruleFactory.GetRules(currentTime)
	for _, action := range actions {
		actionOutput, err := action.Execute(
			ctx,
			rules,
			tsv,
			currentTime,
			args.Actor,
			ids.Empty,
		)

		actionResult := SimulateActionResult{
			Output: actionOutput,
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

type GetBalanceArgs struct {
	Address codec.Address `json:"address"`
}

type GetBalanceReply struct {
	Balance uint64 `json:"balance"`
}

func (j *JSONRPCServer) GetBalance(
	req *http.Request,
	args *GetBalanceArgs,
	reply *GetBalanceReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.GetBalance")
	defer span.End()

	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}

	balance, err := j.vm.BalanceHandler().GetBalance(ctx, args.Address, im)
	if err != nil {
		return err
	}

	reply.Balance = balance
	return nil
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package jsonrpc

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state/tstate"
)

const (
	Endpoint = "/coreapi"
)

var _ api.HandlerFactory[api.VM] = (*JSONRPCServerFactory)(nil)

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

	actionCodec, authCodec := j.vm.ActionCodec(), j.vm.AuthCodec()
	rtx := codec.NewReader(args.Tx, consts.NetworkSizeLimit) // will likely be much smaller than this
	tx, err := chain.UnmarshalTx(rtx, actionCodec, authCodec)
	if err != nil {
		return fmt.Errorf("%w: unable to unmarshal on public service", err)
	}
	if !rtx.Empty() {
		return errors.New("tx has extra bytes")
	}
	if err := tx.Verify(ctx); err != nil {
		return err
	}
	txID := tx.ID()
	reply.TxID = txID
	return j.vm.Submit(ctx, false, []*chain.Transaction{tx})[0]
}

type LastAcceptedReply struct {
	Height    uint64 `json:"height"`
	BlockID   ids.ID `json:"blockId"`
	Timestamp int64  `json:"timestamp"`
}

func (j *JSONRPCServer) LastAccepted(_ *http.Request, _ *struct{}, reply *LastAcceptedReply) error {
	blk := j.vm.LastAcceptedBlock()
	reply.Height = blk.Hght
	reply.BlockID = blk.ID()
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
	actionCodec, outputCodec := j.vm.ActionCodec(), j.vm.OutputCodec()
	vmABI, err := abi.NewABI(actionCodec.GetRegisteredTypes(), outputCodec.GetRegisteredTypes())
	if err != nil {
		return err
	}
	reply.ABI = vmABI
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

func (j *JSONRPCServer) Execute(
	req *http.Request,
	args *ExecuteActionArgs,
	reply *ExecuteActionReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.ExecuteAction")
	defer span.End()

	actionCodec := j.vm.ActionCodec()
	actions := make([]chain.Action, 0)
	for _, action := range args.Actions {
		action, err := actionCodec.Unmarshal(codec.NewReader(action, len(action)))
		if err != nil {
			return fmt.Errorf("failed to unmashal action: %w", err)
		}
		actions = append(actions, action)
	}

	now := time.Now().UnixMilli()

	storage := make(map[string][]byte)
	ts := tstate.New(1)

	for _, action := range actions {
		// Get expected state keys
		stateKeysWithPermissions := action.StateKeys(args.Actor)

		// flatten the map to a slice of keys
		storageKeysToRead := make([][]byte, 0)
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

		tsv := ts.NewView(stateKeysWithPermissions, storage)

		output, err := action.Execute(
			ctx,
			j.vm.Rules(now),
			tsv,
			now,
			args.Actor,
			ids.Empty,
		)
		if err != nil {
			reply.Error = fmt.Sprintf("failed to execute action: %s", err)
			return nil
		}

		tsv.Commit()

		encodedOutput, err := chain.MarshalTyped(output)
		if err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}

		reply.Outputs = append(reply.Outputs, encodedOutput)
	}
	return nil
}

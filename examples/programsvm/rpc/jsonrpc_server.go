// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"github.com/ava-labs/hypersdk/examples/programsvm/actions"
	"github.com/ava-labs/hypersdk/state"
	"github.com/status-im/keycard-go/hexutils"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/programsvm/consts"
	"github.com/ava-labs/hypersdk/examples/programsvm/genesis"
	"github.com/ava-labs/hypersdk/fees"
)

type JSONRPCServer struct {
	c        Controller
	simulate func(context.Context, actions.CallProgram, codec.Address) (state.Keys, uint64, error)
}

func NewJSONRPCServer(c Controller, simulate func(context.Context, actions.CallProgram, codec.Address) (state.Keys, uint64, error)) *JSONRPCServer {
	return &JSONRPCServer{c, simulate}
}

type GenesisReply struct {
	Genesis *genesis.Genesis `json:"genesis"`
}

func (j *JSONRPCServer) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
	reply.Genesis = j.c.Genesis()
	return nil
}

type SimulateCallProgramTxArgs struct {
	CallProgramTx actions.CallProgram `json:"callProgramTx"`
	Actor         codec.Address       `json:"actor"`
}

type SimulateStateKey struct {
	HexKey      string `json:"hex"`
	Permissions byte   `json:"perm"`
}
type SimulateCallProgramTxReply struct {
	StateKeys    []SimulateStateKey `json:"stateKeys"`
	FuelConsumed uint64             `json:"fuel"`
}

func (j *JSONRPCServer) SimulateCallProgramTx(req *http.Request, args *SimulateCallProgramTxArgs, reply *SimulateCallProgramTxReply) (err error) {
	stateKeys, fuelConsumed, err := j.simulate(req.Context(), args.CallProgramTx, args.Actor)
	if err != nil {
		return err
	}
	reply.StateKeys = make([]SimulateStateKey, 0, len(stateKeys))
	for key, permission := range stateKeys {
		reply.StateKeys = append(reply.StateKeys, SimulateStateKey{HexKey: hexutils.BytesToHex([]byte(key)), Permissions: byte(permission)})
	}
	reply.FuelConsumed = fuelConsumed
	return nil
}

type TxArgs struct {
	TxID ids.ID `json:"txId"`
}

type TxReply struct {
	Timestamp int64           `json:"timestamp"`
	Success   bool            `json:"success"`
	Units     fees.Dimensions `json:"units"`
	Fee       uint64          `json:"fee"`
}

func (j *JSONRPCServer) Tx(req *http.Request, args *TxArgs, reply *TxReply) error {
	_, span := j.c.Tracer().Start(req.Context(), "Server.Tx")
	defer span.End()

	found, t, success, units, fee, err := j.c.GetTransaction(args.TxID)
	if err != nil {
		return err
	}
	if !found {
		return ErrTxNotFound
	}
	reply.Timestamp = t
	reply.Success = success
	reply.Units = units
	reply.Fee = fee
	return nil
}

type BalanceArgs struct {
	Address string `json:"address"`
}

type BalanceReply struct {
	Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) Balance(req *http.Request, args *BalanceArgs, reply *BalanceReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.Balance")
	defer span.End()

	addr, err := codec.ParseAddressBech32(consts.HRP, args.Address)
	if err != nil {
		return err
	}
	balance, err := j.c.GetBalanceFromState(ctx, addr)
	if err != nil {
		return err
	}
	reply.Amount = balance
	return err
}

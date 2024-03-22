// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/shim"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state"
)

type JSONRPCServer struct {
	c Controller
}

func NewJSONRPCServer(c Controller) *JSONRPCServer {
	return &JSONRPCServer{c}
}

type GenesisReply struct {
	Genesis *genesis.Genesis `json:"genesis"`
}

func (j *JSONRPCServer) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
	reply.Genesis = j.c.Genesis()
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
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.Tx")
	defer span.End()

	found, t, success, units, fee, err := j.c.GetTransaction(ctx, args.TxID)
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

type EVMAccountArgs struct {
	Address string `json:"address"`
}

type EVMAccountReply struct {
	Nonce   uint64 `json:"nonce"`
	Balance uint64 `json:"balance"`
}

func (j *JSONRPCServer) EvmAccount(req *http.Request, args *EVMAccountArgs, reply *EVMAccountReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.EvmAccount")
	defer span.End()

	addr := common.HexToAddress(args.Address)
	view, err := j.c.View()
	if err != nil {
		return err
	}
	mu := state.NewSimpleMutable(view)
	statedb := shim.NewStateDB(ctx, mu)
	reply.Balance = statedb.GetBalance(addr).Uint64()
	reply.Nonce = statedb.GetNonce(addr)
	return nil
}

type EVMGetCodeArgs struct {
	Address string `json:"address"`
}

type EVMGetCodeReply struct {
	Code []byte `json:"code"`
}

func (j *JSONRPCServer) EvmGetCode(req *http.Request, args *EVMGetCodeArgs, reply *EVMGetCodeReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.EvmGetCode")
	defer span.End()

	addr := common.HexToAddress(args.Address)
	view, err := j.c.View()
	if err != nil {
		return err
	}
	mu := state.NewSimpleMutable(view)
	statedb := shim.NewStateDB(ctx, mu)
	reply.Code = statedb.GetCode(addr)
	return nil
}

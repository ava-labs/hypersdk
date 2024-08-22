// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"net/http"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/vm"
)

const JSONRPCEndpoint = "/morpheusapi"

var _ api.HandlerFactory[vm.Controller] = (*jsonRPCServerFactory)(nil)

type jsonRPCServerFactory struct{}

func (jsonRPCServerFactory) New(c vm.Controller) (api.Handler, error) {
	handler, err := api.NewJSONRPCHandler(consts.Name, NewJSONRPCServer(c))
	return api.Handler{
		Path:    JSONRPCEndpoint,
		Handler: handler,
	}, err
}

type JSONRPCServer struct {
	c *Controller
}

func NewJSONRPCServer(c vm.Controller) *JSONRPCServer {
	return &JSONRPCServer{c: c.(*Controller)}
}

type GenesisReply struct {
	Genesis *vm.Bech32Genesis `json:"genesis"`
}

func (j *JSONRPCServer) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
	reply.Genesis = j.c.inner.Genesis().(*vm.Bech32Genesis)
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

type RulesReply struct {
	Rules *vm.Rules `json:"rules"`
}

func (j *JSONRPCServer) Rules(_ *http.Request, _ *struct{}, reply *RulesReply) (err error) {
	reply.Rules = j.c.inner.Rules(0).(*vm.Rules)
	return nil
}

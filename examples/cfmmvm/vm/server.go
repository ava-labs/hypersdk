// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"net/http"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/genesis"
)

const JSONRPCEndpoint = "/cfmmapi"

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

type GetTokenInfoArgs struct {
	TokenAddress codec.Address `json:"tokenAddress"`
}

type GetTokenInfoReply struct {
	Name        string        `json:"name"`
	Symbol      string        `json:"symbol"`
	Metadata    string        `json:"metadata"`
	TotalSupply uint64        `json:"totalSupply"`
	Owner       codec.Address `json:"owner"`
}

func (j *JSONRPCServer) GetTokenInfo(req *http.Request, args *GetTokenInfoArgs, reply *GetTokenInfoReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.GetTokenInfo")
	defer span.End()

	name, symbol, metadata, totalSupply, owner, err := storage.GetTokenInfoFromState(ctx, j.vm.ReadState, args.TokenAddress)
	if err != nil {
		return err
	}
	reply.Name = string(name)

	reply.Symbol = string(symbol)
	reply.Metadata = string(metadata)
	reply.TotalSupply = totalSupply
	reply.Owner = owner
	return nil
}

type GetBalanceArgs struct {
	TokenAddress codec.Address `json:"tokenAddress"`
	Account      codec.Address `json:"account"`
}

type GetBalanceReply struct {
	Balance uint64 `json:"balance"`
}

func (j *JSONRPCServer) GetBalance(req *http.Request, args *GetBalanceArgs, reply *GetBalanceReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.GetBalance")
	defer span.End()

	balance, err := storage.GetTokenAccountBalanceFromState(ctx, j.vm.ReadState, args.TokenAddress, args.Account)
	if err != nil {
		return err
	}
	reply.Balance = balance
	return nil
}

type GetLiquidityPoolArgs struct {
	LiquidityPoolAddress codec.Address `json:"liquidityPoolAddress"`
}

type GetLiquidityPoolReply struct {
	TokenX         codec.Address `json:"tokenX"`
	TokenY         codec.Address `json:"tokenY"`
	Fee            uint64        `json:"fee"`
	FeeTo          codec.Address `json:"feeTo"`
	FunctionID     uint8         `json:"functionID"`
	ReserveX       uint64        `json:"reserveX"`
	ReserveY       uint64        `json:"reserveY"`
	LiquidityToken codec.Address `json:"liquidityToken"`
	KLast          uint64        `json:"kLast"`
}

func (j *JSONRPCServer) GetLiquidityPool(req *http.Request, args *GetLiquidityPoolArgs, reply *GetLiquidityPoolReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.GetLiquidityPool")
	defer span.End()

	functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, liquidityToken, kLast, err := storage.GetLiquidityPoolFromState(ctx, j.vm.ReadState, args.LiquidityPoolAddress)
	if err != nil {
		return err
	}

	reply.TokenX = tokenX
	reply.TokenY = tokenY
	reply.Fee = fee
	reply.FeeTo = feeTo
	reply.FunctionID = functionID
	reply.ReserveX = reserveX
	reply.ReserveY = reserveY
	reply.LiquidityToken = liquidityToken
	reply.KLast = kLast
	return nil
}

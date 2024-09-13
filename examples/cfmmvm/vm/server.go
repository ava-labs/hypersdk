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
	TokenAddress string `json:"tokenAddress"`
}

type GetTokenInfoReply struct {
	Name        string `json:"name"`
	Symbol      string `json:"symbol"`
	Metadata    string `json:"metadata"`
	TotalSupply uint64 `json:"totalSupply"`
	Owner       string `json:"owner"`
}

func (j *JSONRPCServer) GetTokenInfo(req *http.Request, args *GetTokenInfoArgs, reply *GetTokenInfoReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.GetTokenInfo")
	defer span.End()

	addr, err := codec.ParseAddressBech32(consts.HRP, args.TokenAddress)
	if err != nil {
		return err
	}

	name, symbol, metadata, totalSupply, owner, err := storage.GetTokenInfoFromState(ctx, j.vm.ReadState, addr)
	if err != nil {
		return err
	}
	reply.Name = string(name)

	reply.Symbol = string(symbol)
	reply.Metadata = string(metadata)
	reply.TotalSupply = totalSupply
	ownerStr, err := codec.AddressBech32(consts.HRP, owner)
	if err != nil {
		return err
	}
	reply.Owner = ownerStr
	return nil
}

type GetBalanceArgs struct {
	TokenAddress string `json:"tokenAddress"`
	Account      string `json:"account"`
}

type GetBalanceReply struct {
	Balance uint64 `json:"balance"`
}

func (j *JSONRPCServer) GetBalance(req *http.Request, args *GetBalanceArgs, reply *GetBalanceReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.GetBalance")
	defer span.End()

	accountAddr, err := codec.ParseAddressBech32(consts.HRP, args.Account)
	if err != nil {
		return err
	}
	tokenAddr, err := codec.ParseAddressBech32(consts.HRP, args.TokenAddress)
	if err != nil {
		return err
	}

	balance, err := storage.GetTokenAccountBalanceFromState(ctx, j.vm.ReadState, tokenAddr, accountAddr)
	if err != nil {
		return err
	}
	reply.Balance = balance
	return nil
}

type GetLiquidityPoolArgs struct {
	LiquidityPoolAddress string `json:"liquidityPoolAddress"`
}

type GetLiquidityPoolReply struct {
	TokenX         string `json:"tokenX"`
	TokenY         string `json:"tokenY"`
	Fee            uint64 `json:"fee"`
	FeeTo          string `json:"feeTo"`
	FunctionID     uint8  `json:"functionID"`
	ReserveX       uint64 `json:"reserveX"`
	ReserveY       uint64 `json:"reserveY"`
	LiquidityToken string `json:"liquidityToken"`
	KLast          uint64 `json:"kLast"`
}

func (j *JSONRPCServer) GetLiquidityPool(req *http.Request, args *GetLiquidityPoolArgs, reply *GetLiquidityPoolReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.GetLiquidityPool")
	defer span.End()

	poolAddr, err := codec.ParseAddressBech32(consts.HRP, args.LiquidityPoolAddress)
	if err != nil {
		return err
	}

	functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, liquidityToken, kLast, err := storage.GetLiquidityPoolFromState(ctx, j.vm.ReadState, poolAddr)
	if err != nil {
		return err
	}

	tokenXStr, err := codec.AddressBech32(consts.HRP, tokenX)
	if err != nil {
		return err
	}
	reply.TokenX = tokenXStr

	tokenYStr, err := codec.AddressBech32(consts.HRP, tokenY)
	if err != nil {
		return err
	}
	reply.TokenY = tokenYStr
	reply.Fee = fee

	feeToStr, err := codec.AddressBech32(consts.HRP, feeTo)
	if err != nil {
		return err
	}
	reply.FeeTo = feeToStr
	reply.FunctionID = functionID
	reply.ReserveX = reserveX
	reply.ReserveY = reserveY

	liquidityTokenStr, err := codec.AddressBech32(consts.HRP, liquidityToken)
	if err != nil {
		return err
	}
	reply.LiquidityToken = liquidityTokenStr
	reply.KLast = kLast
	return nil
}

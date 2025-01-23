// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"net/http"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ethereum/go-ethereum/common"
)

const JSONRPCEndpoint = "/morpheusapi"

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
	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	evmAddr := common.Address{}
	copy(evmAddr[:], args.Address[13:])
	balance, err := storage.GetBalance(ctx, im, evmAddr)
	if err != nil {
		return err
	}
	reply.Amount = balance
	return err
}

type NonceArgs struct {
	Address codec.Address `json:"address"`
}

type NonceReply struct {
	Nonce uint64 `json:"nonce"`
}

func (j *JSONRPCServer) Nonce(req *http.Request, args *NonceArgs, reply *NonceReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Nonce")
	defer span.End()
	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	nonce, err := storage.GetNonce(ctx, im, storage.ConvertAddress(args.Address))
	if err != nil {
		return err
	}
	reply.Nonce = nonce
	return nil
}

type EVMGetCodeArgs struct {
	Address common.Address `json:"address"`
}

type EVMGetCodeReply struct {
	Code []byte `json:"code"`
}

func (j *JSONRPCServer) EvmGetCode(req *http.Request, args *EVMGetCodeArgs, reply *EVMGetCodeReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Nonce")
	defer span.End()
	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	code, err := storage.GetCode(ctx, im, args.Address)
	if err != nil {
		return err
	}
	reply.Code = code
	return nil
}

type GetBalanceEVMArgs struct {
	Address common.Address
}

type GetBalanceEVMReply struct {
	Balance uint64
}

func (j *JSONRPCServer) GetBalanceEVM(
	req *http.Request,
	args *GetBalanceEVMArgs,
	reply *GetBalanceEVMReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.GetBalanceEVM")
	defer span.End()
	im, err := j.vm.ImmutableState(ctx)
	if err != nil {
		return err
	}
	balance, err := storage.GetBalance(ctx, im, args.Address)
	if err != nil {
		return err
	}
	reply.Balance = balance
	return nil
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"net/http"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/testvm/consts"
	"github.com/ava-labs/hypersdk/examples/testvm/storage"
	"github.com/ava-labs/hypersdk/genesis"
)

const JSONRPCEndpoint = "/testvmapi"

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

type CountArgs struct {
	Address codec.Address `json:"address"`
}

type CountReply struct {
	Count uint64 `json:"count"`
}

func (j *JSONRPCServer) Count(req *http.Request, args *CountArgs, reply *CountReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "Server.Count")
	defer span.End()

	count, err := storage.GetCounterFromState(ctx, j.vm.ReadState, args.Address)
	if err != nil {
		return err
	}
	reply.Count = count
	return err
}

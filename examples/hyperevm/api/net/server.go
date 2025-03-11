// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package net

import (
	"math/big"
	"net/http"

	"github.com/ava-labs/subnet-evm/params"

	"github.com/ava-labs/hypersdk/api"
)

const Namespace = "net"

type NetServer struct {
	vm api.VM
}

func NewService(vm api.VM) (*NetServer, string) {
	return NewNetServer(vm), Namespace
}

func NewNetServer(vm api.VM) *NetServer {
	return &NetServer{vm: vm}
}

func (*NetServer) Version(_ *http.Request, _ *struct{}, reply *uint64) error {
	chainID := big.NewInt(0).Add(params.DefaultChainID, big.NewInt(1))
	*reply = chainID.Uint64()

	return nil
}

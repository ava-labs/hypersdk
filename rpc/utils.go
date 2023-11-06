// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"net/http"

	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/gorilla/rpc/v2"
)

func NewJSONRPCHandler(
	name string,
	service interface{},
) (http.Handler, error) {
	server := rpc.NewServer()
	server.RegisterCodec(json.NewCodec(), "application/json")
	server.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	return server, server.RegisterService(service, name)
}

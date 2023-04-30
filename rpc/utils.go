// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"net/http"

	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/gorilla/rpc/v2"
)

func NewJSONRPCHandler(
	name string,
	service interface{},
	lockOption common.LockOption,
) (*common.HTTPHandler, error) {
	server := rpc.NewServer()
	server.RegisterCodec(json.NewCodec(), "application/json")
	server.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	if err := server.RegisterService(service, name); err != nil {
		return nil, err
	}
	return &common.HTTPHandler{LockOptions: lockOption, Handler: server}, nil
}

func NewWebSocketHandler(server http.Handler) *common.HTTPHandler {
	return &common.HTTPHandler{LockOptions: common.NoLock, Handler: server}
}

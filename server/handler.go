// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package server

import (
	"net/http"

	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/gorilla/rpc/v2"
)

func NewHandler(service any, name string) (http.Handler, error) {
	newServer := rpc.NewServer()
	codec := json.NewCodec()
	newServer.RegisterCodec(codec, "application/json")
	newServer.RegisterCodec(codec, "application/json;charset=UTF-8")
	if err := newServer.RegisterService(service, name); err != nil {
		return nil, err
	}
	return newServer, nil
}

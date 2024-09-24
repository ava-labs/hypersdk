// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"net/http"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
)

const Endpoint = "/corestate"

var _ api.HandlerFactory[api.VM[chain.PendingView]] = (*JSONRPCStateServerFactory[chain.PendingView])(nil)

type JSONRPCStateServerFactory[T chain.PendingView] struct{}

func (JSONRPCStateServerFactory[T]) New(stateReader api.VM[T]) (api.Handler, error) {
	handler, err := api.NewJSONRPCHandler(api.Name, NewJSONRPCStateServer(stateReader))
	if err != nil {
		return api.Handler{}, err
	}

	return api.Handler{
		Path:    Endpoint,
		Handler: handler,
	}, nil
}

type ReadStateRequest struct {
	Keys [][]byte
}

type ReadStateResponse struct {
	Values [][]byte
	Errors []string
}

func NewJSONRPCStateServer[T chain.PendingView](stateReader api.VM[T]) *JSONRPCStateServer[T] {
	return &JSONRPCStateServer[T]{
		stateReader: stateReader,
	}
}

// JSONRPCStateServer gives direct read access to the vm state
type JSONRPCStateServer[T chain.PendingView] struct {
	stateReader api.VM[T]
}

func (s *JSONRPCStateServer[_]) ReadState(req *http.Request, args *ReadStateRequest, res *ReadStateResponse) error {
	ctx, span := s.stateReader.Tracer().Start(req.Context(), "Server.ReadState")
	defer span.End()

	var errs []error
	res.Values, errs = s.stateReader.ReadState(ctx, args.Keys)
	for _, err := range errs {
		res.Errors = append(res.Errors, err.Error())
	}
	return nil
}

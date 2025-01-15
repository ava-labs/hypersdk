// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"net/http"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/api"
)

const Endpoint = "/corestate"

var _ api.HandlerFactory[api.VM] = (*JSONRPCStateServerFactory)(nil)

type JSONRPCStateServerFactory struct{}

func (JSONRPCStateServerFactory) New(stateReader api.VM) (api.Handler, error) {
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

func NewJSONRPCStateServer(stateReader api.VM) *JSONRPCStateServer {
	return &JSONRPCStateServer{
		stateReader: stateReader,
	}
}

// JSONRPCStateServer gives direct read access to the vm state
type JSONRPCStateServer struct {
	stateReader api.VM
}

func (s *JSONRPCStateServer) ReadState(req *http.Request, args *ReadStateRequest, res *ReadStateResponse) error {
	ctx, span := s.stateReader.Tracer().Start(req.Context(), "Server.ReadState")
	defer span.End()

	var errs []error
	res.Values, errs = s.stateReader.ReadState(ctx, args.Keys)
	res.Errors = make([]string, len(errs))
	for i, err := range errs {
		if err != nil {
			res.Errors[i] = err.Error()
		}
		if err != nil && err != database.ErrNotFound {
			return err
		}
	}
	return nil
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"net/http"

	"github.com/ava-labs/avalanchego/trace"
)

type stateReader interface {
	Tracer() trace.Tracer
	ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error)
}

type StateRequest struct {
	Keys [][]byte
}

type StateResponse struct {
	Values [][]byte
	Errors []error
}

func NewJSONRPCStateServer(stateReader stateReader) *JSONRPCStateServer {
	return &JSONRPCStateServer{
		stateReader: stateReader,
	}
}

// JSONRPCStateServer gives direct read access to the vm state
type JSONRPCStateServer struct {
	stateReader
}

func (s *JSONRPCStateServer) ReadState(req *http.Request, args *StateRequest, res *StateResponse) error {
	ctx, span := s.stateReader.Tracer().Start(req.Context(), "Server.ReadState")
	defer span.End()
	res.Values, res.Errors = s.stateReader.ReadState(ctx, args.Keys)
	return nil
}

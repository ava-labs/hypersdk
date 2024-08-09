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
	_, span := s.stateReader.Tracer().Start(req.Context(), "Server.ReadState")
	defer span.End()
	res.Values, res.Errors = s.stateReader.ReadState(req.Context(), args.Keys)
	return nil
}

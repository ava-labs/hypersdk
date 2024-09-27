// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txindexer

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
)

const Endpoint = "/blockindexer"

var (
	ErrNoLatestBlock = errors.New("latest block not found")

	_ api.HandlerFactory[api.VM] = (*apiFactory)(nil)
)

type apiFactory struct {
	path    string
	name    string
	indexer *indexer
}

func (f *apiFactory) New(vm api.VM) (api.Handler, error) {
	handler, err := api.NewJSONRPCHandler(f.name, &Server{
		tracer:  vm.Tracer(),
		indexer: f.indexer,
	})
	if err != nil {
		return api.Handler{}, err
	}

	return api.Handler{
		Path:    f.path,
		Handler: handler,
	}, nil
}

type Server struct {
	tracer  trace.Tracer
	indexer *indexer
}

type GetBlockRequest struct {
	BlockID ids.ID `json:"blockID"`
}

type GetBlockByHeightRequest struct {
	Height uint64 `json:"height"`
}

type GetBlockResponse struct {
	Block *chain.StatefulBlock `json:"block"`
}

func (s *Server) GetBlock(req *http.Request, args *GetBlockRequest, reply *GetBlockResponse) error {
	_, span := s.tracer.Start(req.Context(), "Indexer.GetBlock")
	defer span.End()

	block, ok := s.indexer.GetBlock(args.BlockID)
	if !ok {
		return fmt.Errorf("block %s not found", args.BlockID)
	}
	reply.Block = block
	return nil
}

func (s *Server) GetBlockByHeight(req *http.Request, args *GetBlockByHeightRequest, reply *GetBlockResponse) error {
	_, span := s.tracer.Start(req.Context(), "Indexer.GetBlockByHeight")
	defer span.End()

	block, ok := s.indexer.GetBlockByHeight(args.Height)
	if !ok {
		return fmt.Errorf("block at height %d not found", args.Height)
	}
	reply.Block = block
	return nil
}

func (s *Server) GetLatestBlock(req *http.Request, _ *struct{}, reply *GetBlockResponse) error {
	_, span := s.tracer.Start(req.Context(), "Indexer.GetLatestBlock")
	defer span.End()

	block, ok := s.indexer.GetLatestBlock()
	if !ok {
		return ErrNoLatestBlock
	}
	reply.Block = block
	return nil
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"
)

const Endpoint = "/indexer"

var (
	ErrTxNotFound    = errors.New("tx not found")
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

type GetTxRequest struct {
	TxID ids.ID `json:"txId"`
}

type GetTxResponse struct {
	Timestamp int64           `json:"timestamp"`
	Success   bool            `json:"success"`
	Units     fees.Dimensions `json:"units"`
	Fee       uint64          `json:"fee"`
}

type Server struct {
	tracer  trace.Tracer
	indexer *indexer
}

func (s *Server) GetTx(req *http.Request, args *GetTxRequest, reply *GetTxResponse) error {
	_, span := s.tracer.Start(req.Context(), "Indexer.GetTx")
	defer span.End()

	found, t, success, units, fee, err := s.indexer.GetTransaction(args.TxID)
	if err != nil {
		return err
	}

	if !found {
		return ErrTxNotFound
	}
	reply.Timestamp = t
	reply.Success = success
	reply.Units = units
	reply.Fee = fee
	return nil
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

	block, ok := s.indexer.getBlock(args.BlockID)
	if !ok {
		return fmt.Errorf("block %s not found", args.BlockID)
	}
	reply.Block = block
	return nil
}

func (s *Server) GetBlockByHeight(req *http.Request, args *GetBlockByHeightRequest, reply *GetBlockResponse) error {
	_, span := s.tracer.Start(req.Context(), "Indexer.GetBlockByHeight")
	defer span.End()

	block, ok := s.indexer.getBlockByHeight(args.Height)
	if !ok {
		return fmt.Errorf("block at height %d not found", args.Height)
	}
	reply.Block = block
	return nil
}

func (s *Server) GetLatestBlock(req *http.Request, _ *struct{}, reply *GetBlockResponse) error {
	_, span := s.tracer.Start(req.Context(), "Indexer.GetLatestBlock")
	defer span.End()

	block, ok := s.indexer.getLatestBlock()
	if !ok {
		return ErrNoLatestBlock
	}
	reply.Block = block
	return nil
}

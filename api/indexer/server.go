// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"errors"
	"math"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

const Endpoint = "/indexer"

var (
	ErrTxNotFound = errors.New("tx not found")

	_ api.HandlerFactory[api.VM] = (*apiFactory)(nil)
)

type apiFactory struct {
	path    string
	name    string
	indexer *Indexer
}

func (f *apiFactory) New(vm api.VM) (api.Handler, error) {
	handler, err := api.NewJSONRPCHandler(f.name, NewServer(
		vm.Tracer(),
		f.indexer,
	))
	if err != nil {
		return api.Handler{}, err
	}

	return api.Handler{
		Path:    f.path,
		Handler: handler,
	}, nil
}

func NewServer(tracer trace.Tracer, indexer *Indexer) *Server {
	return &Server{
		tracer:  tracer,
		indexer: indexer,
	}
}

type Server struct {
	tracer  trace.Tracer
	indexer *Indexer
}

type GetBlockRequest struct {
	BlockID     ids.ID `json:"blockID"`
	BlockNumber uint64 `json:"blockNumber"`
}

type GetBlockResponse struct {
	Block      *chain.ExecutedBlock `json:"block"`
	BlockBytes codec.Bytes          `json:"blockBytes"`
}

func (s *Server) GetBlock(req *http.Request, args *GetBlockRequest, reply *GetBlockResponse) error {
	_, span := s.tracer.Start(req.Context(), "Indexer.GetBlock")
	defer span.End()

	var (
		executedBlk *chain.ExecutedBlock
		err         error
	)
	switch {
	case args.BlockID != ids.Empty:
		executedBlk, err = s.indexer.GetBlock(args.BlockID)
	case args.BlockNumber != math.MaxUint64:
		// use the block number.
		executedBlk, err = s.indexer.GetBlockByHeight(args.BlockNumber)
	default:
		// get the latest block.
		executedBlk, err = s.indexer.GetLatestBlock()
	}
	if err != nil {
		return err
	}
	executedBlkBytes, err := executedBlk.Marshal()
	if err != nil {
		return err
	}
	reply.Block = executedBlk
	reply.BlockBytes = executedBlkBytes
	return nil
}

type GetTxRequest struct {
	TxID ids.ID `json:"txId"`
}

type GetTxResponse struct {
	TxBytes   codec.Bytes   `json:"transactionBytes"`
	Timestamp int64         `json:"timestamp"`
	Result    *chain.Result `json:"result"`
}

func (s *Server) GetTx(req *http.Request, args *GetTxRequest, reply *GetTxResponse) error {
	_, span := s.tracer.Start(req.Context(), "Indexer.GetTx")
	defer span.End()

	found, tx, t, result, err := s.indexer.GetTransaction(args.TxID)
	if err != nil {
		return err
	}

	if !found {
		return ErrTxNotFound
	}
	reply.Timestamp = t
	reply.TxBytes = tx.Bytes()
	reply.Result = result
	return nil
}

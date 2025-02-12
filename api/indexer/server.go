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
	"github.com/ava-labs/hypersdk/fees"
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

type GetBlockRequest struct {
	BlockID     ids.ID `json:"blockID"`
	BlockNumber uint64 `json:"blockNumber"`
}

type GetBlockResponse struct {
	Block      *chain.ExecutedBlock `json:"block"`
	BlockBytes codec.Bytes          `json:"blockBytes"`
}

func (g *GetBlockResponse) setResponse(block *chain.ExecutedBlock) error {
	g.Block = block
	blockBytes, err := block.Marshal()
	if err != nil {
		return err
	}
	g.BlockBytes = blockBytes
	return nil
}

func (s *Server) GetBlock(req *http.Request, args *GetBlockRequest, reply *GetBlockResponse) error {
	_, span := s.tracer.Start(req.Context(), "Indexer.GetBlock")
	defer span.End()

	var executedBlk *chain.ExecutedBlock
	var err error
	if args.BlockID != ids.Empty {
		executedBlk, err = s.indexer.GetBlock(args.BlockID)
	} else if args.BlockNumber != math.MaxUint64 {
		// use the block number.
		executedBlk, err = s.indexer.GetBlockByHeight(args.BlockNumber)
	} else {
		// get the latest block.
		executedBlk, err = s.indexer.GetLatestBlock()
	}
	if err != nil {
		return err
	}
	return reply.setResponse(executedBlk)
}

type GetTxRequest struct {
	TxID ids.ID `json:"txId"`
}

type GetTxResponse struct {
	Timestamp int64           `json:"timestamp"`
	Success   bool            `json:"success"`
	Units     fees.Dimensions `json:"units"`
	Fee       uint64          `json:"fee"`
	Outputs   []codec.Bytes   `json:"result"`
	ErrorStr  string          `json:"errorStr"`
}

type Server struct {
	tracer  trace.Tracer
	indexer *Indexer
}

func (s *Server) GetTx(req *http.Request, args *GetTxRequest, reply *GetTxResponse) error {
	_, span := s.tracer.Start(req.Context(), "Indexer.GetTx")
	defer span.End()

	found, t, success, units, fee, outputs, errorStr, err := s.indexer.GetTransaction(args.TxID)
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
	wrappedOutputs := make([]codec.Bytes, len(outputs))
	for i, output := range outputs {
		wrappedOutputs[i] = codec.Bytes(output)
	}
	reply.Outputs = wrappedOutputs
	reply.ErrorStr = errorStr
	return nil
}

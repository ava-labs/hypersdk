// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"errors"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/rpc"
)

var ErrTxNotFound = errors.New("tx not found")

func NewAPIFactory(indexer *TxDBIndexer, name string, path string) *APIFactory {
	return &APIFactory{
		path:    path,
		name:    name,
		indexer: indexer,
	}
}

type APIFactory struct {
	path    string
	name    string
	indexer *TxDBIndexer
}

func (f *APIFactory) New(vm rpc.VM) (rpc.HTTPHandler, error) {
	handler, err := rpc.NewJSONRPCHandler(f.name, &Server{
		tracer:  vm.Tracer(),
		indexer: f.indexer,
	})
	if err != nil {
		return rpc.HTTPHandler{}, err
	}

	return rpc.HTTPHandler{
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
	indexer *TxDBIndexer
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

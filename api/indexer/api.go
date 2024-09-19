// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"errors"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"
)

const Endpoint = "/indexer"

var (
	ErrTxNotFound = errors.New("tx not found")

	_ api.HandlerFactory[api.VM[chain.RuntimeInterface]] = (*apiFactory[chain.RuntimeInterface])(nil)
)

type apiFactory[T chain.RuntimeInterface] struct {
	path    string
	name    string
	indexer *txDBIndexer[T]
}

func (f *apiFactory[T]) New(vm api.VM[T]) (api.Handler, error) {
	handler, err := api.NewJSONRPCHandler(f.name, &Server[T]{
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

type Server[T chain.RuntimeInterface] struct {
	tracer  trace.Tracer
	indexer *txDBIndexer[T]
}

func (s *Server[_]) GetTx(req *http.Request, args *GetTxRequest, reply *GetTxResponse) error {
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

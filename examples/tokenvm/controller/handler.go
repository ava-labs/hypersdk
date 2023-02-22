// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

type Handler struct {
	*vm.Handler // embed standard functionality

	c *Controller
}

type GenesisReply struct {
	Genesis *genesis.Genesis `json:"genesis"`
}

func (h *Handler) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
	reply.Genesis = h.c.genesis
	return nil
}

type GetTxArgs struct {
	TxID ids.ID `json:"txId"`
}

type GetTxReply struct {
	Accepted bool `json:"accepted"`

	Timestamp int64  `json:"timestamp"`
	Success   bool   `json:"success"`
	Units     uint64 `json:"units"`
}

func (h *Handler) GetTx(req *http.Request, args *GetTxArgs, reply *GetTxReply) error {
	ctx, span := h.c.inner.Tracer().Start(req.Context(), "Handler.GetTx")
	defer span.End()

	accepted, t, success, units, err := storage.GetTransaction(ctx, h.c.metaDB, args.TxID)
	if err != nil {
		return err
	}
	reply.Accepted = accepted
	reply.Timestamp = t
	reply.Success = success
	reply.Units = units
	return nil
}

type BalanceArgs struct {
	Address string `json:"address"`
	Asset   ids.ID `json:"asset"`
}

type BalanceReply struct {
	Amount uint64 `json:"amount"`
}

func (h *Handler) Balance(req *http.Request, args *BalanceArgs, reply *BalanceReply) error {
	ctx, span := h.c.inner.Tracer().Start(req.Context(), "Handler.Balance")
	defer span.End()

	addr, err := utils.ParseAddress(args.Address)
	if err != nil {
		return err
	}
	balance, err := storage.GetBalanceFromState(ctx, h.c.inner.ReadState, addr, args.Asset)
	if err != nil {
		return err
	}
	reply.Amount = balance
	return err
}

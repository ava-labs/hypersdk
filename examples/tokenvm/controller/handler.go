// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"errors"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

const (
	ordersToSend = 128
)

var (
	ErrTxNotFound    = errors.New("tx not found")
	ErrAssetNotFound = errors.New("asset not found")
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

type TxArgs struct {
	TxID ids.ID `json:"txId"`
}

type TxReply struct {
	Timestamp int64  `json:"timestamp"`
	Success   bool   `json:"success"`
	Units     uint64 `json:"units"`
}

func (h *Handler) Tx(req *http.Request, args *TxArgs, reply *TxReply) error {
	ctx, span := h.c.inner.Tracer().Start(req.Context(), "Handler.Tx")
	defer span.End()

	found, t, success, units, err := storage.GetTransaction(ctx, h.c.metaDB, args.TxID)
	if err != nil {
		return err
	}
	if !found {
		return ErrTxNotFound
	}
	reply.Timestamp = t
	reply.Success = success
	reply.Units = units
	return nil
}

type AssetArgs struct {
	Asset ids.ID `json:"asset"`
}

type AssetReply struct {
	Metadata []byte `json:"metadata"`
	Supply   uint64 `json:"supply"`
	Owner    string `json:"owner"`
	Warp     bool   `json:"warp"`
}

func (h *Handler) Asset(req *http.Request, args *AssetArgs, reply *AssetReply) error {
	ctx, span := h.c.inner.Tracer().Start(req.Context(), "Handler.Asset")
	defer span.End()

	exists, metadata, supply, owner, warp, err := storage.GetAssetFromState(
		ctx,
		h.c.inner.ReadState,
		args.Asset,
	)
	if err != nil {
		return err
	}
	if !exists {
		return ErrAssetNotFound
	}
	reply.Metadata = metadata
	reply.Supply = supply
	reply.Owner = utils.Address(owner)
	reply.Warp = warp
	return err
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

type OrdersArgs struct {
	Pair string `json:"pair"`
}

type OrdersReply struct {
	Orders []*Order `json:"orders"`
}

func (h *Handler) Orders(req *http.Request, args *OrdersArgs, reply *OrdersReply) error {
	_, span := h.c.inner.Tracer().Start(req.Context(), "Handler.Orders")
	defer span.End()

	reply.Orders = h.c.orderBook.Orders(args.Pair, ordersToSend)
	return nil
}

type LoanArgs struct {
	Destination ids.ID `json:"destination"`
	Asset       ids.ID `json:"asset"`
}

type LoanReply struct {
	Amount uint64 `json:"amount"`
}

func (h *Handler) Loan(req *http.Request, args *LoanArgs, reply *LoanReply) error {
	ctx, span := h.c.inner.Tracer().Start(req.Context(), "Handler.Loan")
	defer span.End()

	amount, err := storage.GetLoanFromState(ctx, h.c.inner.ReadState, args.Asset, args.Destination)
	if err != nil {
		return err
	}
	reply.Amount = amount
	return nil
}

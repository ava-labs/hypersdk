// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

const (
	Endpoint = "/rpc"
)

type Handler struct {
	vm *VM
}

func (vm *VM) Handler() *Handler {
	return &Handler{vm}
}

type PingReply struct {
	Success bool `json:"success"`
}

func (h *Handler) Ping(_ *http.Request, _ *struct{}, reply *PingReply) (err error) {
	h.vm.snowCtx.Log.Info("ping")
	reply.Success = true
	return nil
}

type NetworkReply struct {
	NetworkID uint32 `json:"networkId"`
	SubnetID  ids.ID `json:"subnetId"`
	ChainID   ids.ID `json:"chainId"`
}

func (h *Handler) Network(_ *http.Request, _ *struct{}, reply *NetworkReply) (err error) {
	reply.NetworkID = h.vm.snowCtx.NetworkID
	reply.SubnetID = h.vm.snowCtx.SubnetID
	reply.ChainID = h.vm.snowCtx.ChainID
	return nil
}

type SubmitTxArgs struct {
	Tx []byte `json:"tx"`
}

type SubmitTxReply struct {
	TxID ids.ID `json:"txId"`
}

func (h *Handler) SubmitTx(req *http.Request, args *SubmitTxArgs, reply *SubmitTxReply) error {
	ctx, span := h.vm.Tracer().Start(req.Context(), "Handler.SubmitTx")
	defer span.End()

	rtx := codec.NewReader(args.Tx, chain.NetworkSizeLimit) // will likely be much smaller than this
	tx, err := chain.UnmarshalTx(rtx, h.vm.actionRegistry, h.vm.authRegistry)
	if err != nil {
		return fmt.Errorf("%w: unable to unmarshal on public service", err)
	}
	if !rtx.Empty() {
		return errors.New("tx has extra bytes")
	}
	sigVerify, err := tx.Init(ctx, h.vm.actionRegistry, h.vm.authRegistry)
	if err != nil {
		return err
	}
	if err := sigVerify(); err != nil {
		return fmt.Errorf("%w: can't init tx", err)
	}
	txID := tx.ID()
	reply.TxID = txID
	return h.vm.Submit(ctx, false, []*chain.Transaction{tx})[0]
}

type LastAcceptedReply struct {
	Height  uint64 `json:"height"`
	BlockID ids.ID `json:"blockId"`
}

func (h *Handler) LastAccepted(_ *http.Request, _ *struct{}, reply *LastAcceptedReply) error {
	blk := h.vm.lastAccepted
	reply.Height = blk.Hght
	reply.BlockID = blk.ID()
	return nil
}

type SuggestedRawFeeReply struct {
	UnitPrice uint64 `json:"unitPrice"`
	BlockCost uint64 `json:"blockCost"`
}

func (h *Handler) SuggestedRawFee(
	req *http.Request,
	_ *struct{},
	reply *SuggestedRawFeeReply,
) error {
	ctx, span := h.vm.Tracer().Start(req.Context(), "Handler.SuggestedRawFee")
	defer span.End()

	unitPrice, blockCost, err := h.vm.SuggestedFee(ctx)
	if err != nil {
		return err
	}
	reply.UnitPrice = unitPrice
	reply.BlockCost = blockCost
	return nil
}

type PortReply struct {
	Port uint16 `json:"port"`
}

func (h *Handler) DecisionsPort(_ *http.Request, _ *struct{}, reply *PortReply) error {
	reply.Port = h.vm.DecisionsPort()
	return nil
}

func (h *Handler) BlocksPort(_ *http.Request, _ *struct{}, reply *PortReply) error {
	reply.Port = h.vm.BlocksPort()
	return nil
}

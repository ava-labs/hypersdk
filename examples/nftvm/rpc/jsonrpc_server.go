// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"net/http"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/nftvm/consts"
	"github.com/ava-labs/hypersdk/examples/nftvm/genesis"
	"github.com/ava-labs/hypersdk/fees"
)

type JSONRPCServer struct {
	c Controller
}

func NewJSONRPCServer(c Controller) *JSONRPCServer {
	return &JSONRPCServer{c}
}

type GenesisReply struct {
	Genesis *genesis.Genesis `json:"genesis"`
}

func (j *JSONRPCServer) Genesis(_ *http.Request, _ *struct{}, reply *GenesisReply) (err error) {
	reply.Genesis = j.c.Genesis()
	return nil
}

type TxArgs struct {
	TxID ids.ID `json:"txId"`
}

type TxReply struct {
	Timestamp int64           `json:"timestamp"`
	Success   bool            `json:"success"`
	Units     fees.Dimensions `json:"units"`
	Fee       uint64          `json:"fee"`
}

func (j *JSONRPCServer) Tx(req *http.Request, args *TxArgs, reply *TxReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.Tx")
	defer span.End()

	found, t, success, units, fee, err := j.c.GetTransaction(ctx, args.TxID)
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

type BalanceArgs struct {
	Address string `json:"address"`
}

type BalanceReply struct {
	Amount uint64 `json:"amount"`
}

func (j *JSONRPCServer) Balance(req *http.Request, args *BalanceArgs, reply *BalanceReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.Balance")
	defer span.End()

	addr, err := codec.ParseAddressBech32(consts.HRP, args.Address)
	if err != nil {
		return err
	}
	balance, err := j.c.GetBalanceFromState(ctx, addr)
	if err != nil {
		return err
	}
	reply.Amount = balance
	return err
}

type GetNFTCollectionArgs struct {
	CollectionAddress string `json:"address"`
}

type GetNFTCollectionReply struct {
	Name            string `json:"name"`
	Symbol          string `json:"symbol"`
	Metadata        string `json:"metadata"`
	NumOfInstances  uint32 `json:"numOfInstances"`
	CollectionOwner string `json:"collectionOwner"`
}

func (j *JSONRPCServer) GetNFTCollection(req *http.Request, args *GetNFTCollectionArgs, reply *GetNFTCollectionReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.GetNFTCollection")
	defer span.End()

	addr, err := codec.ParseAddressBech32(consts.HRP, args.CollectionAddress)
	if err != nil {
		return err
	}

	name, symbol, metadata, numOfInstances, collectionOwner, err := j.c.GetNFTCollection(ctx, addr)
	if err != nil {
		return err
	}

	reply.Name = string(name)
	reply.Symbol = string(symbol)
	reply.Metadata = string(metadata)
	reply.NumOfInstances = numOfInstances
	reply.CollectionOwner = codec.MustAddressBech32(consts.HRP, collectionOwner)

	return err
}

type GetNFTInstanceArgs struct {
	ParentCollectionAddress string `json:"parentCollectionAddress"`
	InstanceNum             uint32 `json:"instanceNum"`
}

type GetNFTInstanceReply struct {
	Owner                 string `json:"owner"`
	Metadata              string `json:"metadata"`
	IsListedOnMarketplace bool   `json:"isListedOnMarketplace"`
}

// Address returned in Bech32 format
func (j *JSONRPCServer) GetNFTInstance(req *http.Request, args *GetNFTInstanceArgs, reply *GetNFTInstanceReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.GetNFTInstance")
	defer span.End()

	addr, err := codec.ParseAddressBech32(consts.HRP, args.ParentCollectionAddress)
	if err != nil {
		return err
	}

	owner, metadata, isListedOnMarketplace, err := j.c.GetNFTInstance(ctx, addr, args.InstanceNum)
	if err != nil {
		return err
	}

	reply.Owner = codec.MustAddressBech32(consts.HRP, owner)
	reply.Metadata = string(metadata)
	reply.IsListedOnMarketplace = isListedOnMarketplace
	return nil
}

type GetMarketplaceOrderArgs struct {
	OrderID string `json:"orderID"`
}

type GetMarketplaceOrderReply struct {
	Price uint64 `json:"price"`
}

func (j *JSONRPCServer) GetMarketplaceOrder(req *http.Request, args *GetMarketplaceOrderArgs, reply *GetMarketplaceOrderReply) error {
	ctx, span := j.c.Tracer().Start(req.Context(), "Server.GetMarketplaceOrder")
	defer span.End()

	orderID, err := ids.FromString(args.OrderID)
	if err != nil {
		return err
	}

	price, err := j.c.GetMarketplaceOrder(ctx, orderID)
	if err != nil {
		return err
	}

	reply.Price = price
	return nil
}

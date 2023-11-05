// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	brpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

var _ cli.Controller = (*Controller)(nil)

type Handler struct {
	h *cli.Handler
}

func NewHandler(h *cli.Handler) *Handler {
	return &Handler{h}
}

func (h *Handler) Root() *cli.Handler {
	return h.h
}

func (h *Handler) DefaultActor() (
	ids.ID, []byte, chain.AuthFactory,
	*rpc.JSONRPCClient, *brpc.JSONRPCClient, error,
) {
	priv, err := h.h.GetDefaultKey(true)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, err
	}
	chainID, uris, err := h.h.GetDefaultChain(true)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, err
	}
	cli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := cli.Network(context.TODO())
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, err
	}
	// For [defaultActor], we always send requests to the first returned URI.
	return chainID, priv, auth.NewED25519Factory(
			priv,
		), cli,
		brpc.NewJSONRPCClient(
			uris[0],
			networkID,
			chainID,
		), nil
}

func (*Handler) GetBalance(
	ctx context.Context,
	cli *brpc.JSONRPCClient,
	addr codec.AddressBytes,
) (uint64, error) {
	saddr, err := codec.Address(consts.HRP, addr)
	if err != nil {
		return 0, err
	}
	balance, err := cli.Balance(ctx, saddr)
	if err != nil {
		return 0, err
	}
	if balance == 0 {
		utils.Outf("{{red}}balance:{{/}} 0 %s\n", consts.Symbol)
		utils.Outf("{{red}}please send funds to %s{{/}}\n", saddr)
		utils.Outf("{{red}}exiting...{{/}}\n")
		return 0, nil
	}
	utils.Outf(
		"{{yellow}}balance:{{/}} %s %s\n",
		utils.FormatBalance(balance, consts.Decimals),
		consts.Symbol,
	)
	return balance, nil
}

type Controller struct {
	databasePath string
}

func NewController(databasePath string) *Controller {
	return &Controller{databasePath}
}

func (c *Controller) DatabasePath() string {
	return c.databasePath
}

func (*Controller) Symbol() string {
	return consts.Symbol
}

func (*Controller) Decimals() uint8 {
	return consts.Decimals
}

func (*Controller) Address(addr codec.AddressBytes) string {
	return codec.MustAddress(consts.HRP, addr)
}

func (*Controller) ParseAddress(addr string) (codec.AddressBytes, error) {
	return codec.ParseAddress(consts.HRP, addr)
}

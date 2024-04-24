// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
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

func (*Handler) GetAssetInfo(
	ctx context.Context,
	cli *trpc.JSONRPCClient,
	addr codec.Address,
	assetID ids.ID,
	checkBalance bool,
) ([]byte, uint8, uint64, ids.ID, error) {
	var sourceChainID ids.ID
	exists, symbol, decimals, metadata, supply, _, err := cli.Asset(ctx, assetID, false)
	if err != nil {
		return nil, 0, 0, ids.Empty, err
	}
	if assetID != ids.Empty {
		if !exists {
			hutils.Outf("{{red}}%s does not exist{{/}}\n", assetID)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil, 0, 0, ids.Empty, nil
		}
		hutils.Outf(
			"{{yellow}}symbol:{{/}} %s {{yellow}}decimals:{{/}} %d {{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d\n",
			symbol,
			decimals,
			metadata,
			supply,
		)
	}
	if !checkBalance {
		return symbol, decimals, 0, sourceChainID, nil
	}
	saddr, err := codec.AddressBech32(consts.HRP, addr)
	if err != nil {
		return nil, 0, 0, ids.Empty, err
	}
	balance, err := cli.Balance(ctx, saddr, assetID)
	if err != nil {
		return nil, 0, 0, ids.Empty, err
	}
	if balance == 0 {
		hutils.Outf("{{red}}balance:{{/}} 0 %s\n", assetID)
		hutils.Outf("{{red}}please send funds to %s{{/}}\n", saddr)
		hutils.Outf("{{red}}exiting...{{/}}\n")
	} else {
		hutils.Outf(
			"{{yellow}}balance:{{/}} %s %s\n",
			hutils.FormatBalance(balance, decimals),
			symbol,
		)
	}
	return symbol, decimals, balance, sourceChainID, nil
}

func (h *Handler) DefaultActor() (
	ids.ID, *cli.PrivateKey, chain.AuthFactory,
	*rpc.JSONRPCClient, *rpc.WebSocketClient, *trpc.JSONRPCClient, error,
) {
	addr, priv, err := h.h.GetDefaultKey(true)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	chainID, uris, err := h.h.GetDefaultChain(true)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	// For [defaultActor], we always send requests to the first returned URI.
	jcli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := jcli.Network(context.TODO())
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	scli, err := rpc.NewWebSocketClient(
		uris[0],
		rpc.DefaultHandshakeTimeout,
		pubsub.MaxPendingMessages,
		pubsub.MaxReadMessageSize,
	)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	return chainID, &cli.PrivateKey{Address: addr, Bytes: priv}, auth.NewED25519Factory(ed25519.PrivateKey(priv)), jcli, scli,
		trpc.NewJSONRPCClient(
			uris[0],
			networkID,
			chainID,
		), nil
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

func (*Controller) Address(addr codec.Address) string {
	return codec.MustAddressBech32(consts.HRP, addr)
}

func (*Controller) ParseAddress(address string) (codec.Address, error) {
	return codec.ParseAddressBech32(consts.HRP, address)
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/consts"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/vm"
	"github.com/ava-labs/hypersdk/internal/pubsub"
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
	ids.ID, *cli.PrivateKey, chain.AuthFactory,
	*jsonrpc.JSONRPCClient, *vm.JSONRPCClient, *ws.WebSocketClient, error,
) {
	addr, priv, err := h.h.GetDefaultKey(true)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	var factory chain.AuthFactory
	switch addr[0] {
	case auth.ED25519ID:
		factory = auth.NewED25519Factory(ed25519.PrivateKey(priv))
	case auth.SECP256R1ID:
		factory = auth.NewSECP256R1Factory(secp256r1.PrivateKey(priv))
	case auth.BLSID:
		p, err := bls.PrivateKeyFromBytes(priv)
		if err != nil {
			return ids.Empty, nil, nil, nil, nil, nil, err
		}
		factory = auth.NewBLSFactory(p)
	default:
		return ids.Empty, nil, nil, nil, nil, nil, ErrInvalidAddress
	}
	chainID, uris, err := h.h.GetDefaultChain(true)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	jcli := jsonrpc.NewJSONRPCClient(uris[0])
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	ws, err := ws.NewWebSocketClient(uris[0], ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
	if err != nil {
		return ids.Empty, nil, nil, nil, nil, nil, err
	}
	// For [defaultActor], we always send requests to the first returned URI.
	return chainID, &cli.PrivateKey{
			Address: addr,
			Bytes:   priv,
		}, factory, jcli,
		vm.NewJSONRPCClient(
			uris[0],
		), ws, nil
}

func (*Handler) GetBalance(
	ctx context.Context,
	cli *vm.JSONRPCClient,
	addr codec.Address,
) (uint64, error) {
	saddr, err := codec.AddressBech32(consts.HRP, addr)
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

func (*Controller) Address(addr codec.Address) string {
	return codec.MustAddressBech32(consts.HRP, addr)
}

func (*Controller) ParseAddress(addr string) (codec.Address, error) {
	return codec.ParseAddressBech32(consts.HRP, addr)
}

func (*Controller) GetParser(uri string) (chain.Parser, error) {
	cli := vm.NewJSONRPCClient(uri)
	return cli.Parser(context.TODO())
}

func (*Controller) HandleTx(tx *chain.Transaction, result *chain.Result) {
	handleTx(tx, result)
}

func (*Controller) LookupBalance(address string, uri string) (uint64, error) {
	cli := vm.NewJSONRPCClient(uri)
	balance, err := cli.Balance(context.TODO(), address)
	return balance, err
}

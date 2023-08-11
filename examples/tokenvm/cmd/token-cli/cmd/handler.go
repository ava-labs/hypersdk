// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/cli"
	hconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
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

func (h *Handler) GetAssetInfo(
	ctx context.Context,
	cli *trpc.JSONRPCClient,
	publicKey ed25519.PublicKey,
	assetID ids.ID,
	checkBalance bool,
) (uint64, ids.ID, error) {
	var sourceChainID ids.ID
	if assetID != ids.Empty {
		exists, metadata, supply, _, warp, err := cli.Asset(ctx, assetID)
		if err != nil {
			return 0, ids.Empty, err
		}
		if !exists {
			hutils.Outf("{{red}}%s does not exist{{/}}\n", assetID)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return 0, ids.Empty, nil
		}
		if warp {
			sourceChainID = ids.ID(metadata[hconsts.IDLen:])
			sourceAssetID := ids.ID(metadata[:hconsts.IDLen])
			hutils.Outf(
				"{{yellow}}sourceChainID:{{/}} %s {{yellow}}sourceAssetID:{{/}} %s {{yellow}}supply:{{/}} %d\n",
				sourceChainID,
				sourceAssetID,
				supply,
			)
		} else {
			hutils.Outf(
				"{{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d {{yellow}}warp:{{/}} %t\n",
				string(metadata),
				supply,
				warp,
			)
		}
	}
	if !checkBalance {
		return 0, sourceChainID, nil
	}
	addr := utils.Address(publicKey)
	balance, err := cli.Balance(ctx, addr, assetID)
	if err != nil {
		return 0, ids.Empty, err
	}
	if balance == 0 {
		hutils.Outf("{{red}}balance:{{/}} 0 %s\n", assetID)
		hutils.Outf("{{red}}please send funds to %s{{/}}\n", addr)
		hutils.Outf("{{red}}exiting...{{/}}\n")
		return 0, sourceChainID, nil
	}
	hutils.Outf(
		"{{yellow}}balance:{{/}} %s %s\n",
		h.h.ValueString(assetID, balance),
		h.h.AssetString(assetID),
	)
	return balance, sourceChainID, nil
}

func (h *Handler) DefaultActor() (
	ids.ID, ed25519.PrivateKey, *auth.ED25519Factory,
	*rpc.JSONRPCClient, *trpc.JSONRPCClient, error,
) {
	priv, err := h.h.GetDefaultKey()
	if err != nil {
		return ids.Empty, ed25519.EmptyPrivateKey, nil, nil, nil, err
	}
	chainID, uris, err := h.h.GetDefaultChain()
	if err != nil {
		return ids.Empty, ed25519.EmptyPrivateKey, nil, nil, nil, err
	}
	cli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := cli.Network(context.TODO())
	if err != nil {
		return ids.Empty, ed25519.EmptyPrivateKey, nil, nil, nil, err
	}
	// For [defaultActor], we always send requests to the first returned URI.
	return chainID, priv, auth.NewED25519Factory(
			priv,
		), cli,
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

func (*Controller) Address(pk ed25519.PublicKey) string {
	return utils.Address(pk)
}

func (*Controller) ParseAddress(address string) (ed25519.PublicKey, error) {
	return utils.ParseAddress(address)
}

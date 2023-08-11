// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

func (h *Handler) GenerateKey() error {
	// TODO: encrypt key
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return err
	}
	if err := h.StoreKey(priv); err != nil {
		return err
	}
	publicKey := priv.PublicKey()
	if err := h.StoreDefaultKey(publicKey); err != nil {
		return err
	}
	utils.Outf(
		"{{green}}created address:{{/}} %s",
		h.c.Address(publicKey),
	)
	return nil
}

func (h *Handler) ImportKey(keyPath string) error {
	priv, err := ed25519.LoadKey(keyPath)
	if err != nil {
		return err
	}
	if err := h.StoreKey(priv); err != nil {
		return err
	}
	publicKey := priv.PublicKey()
	if err := h.StoreDefaultKey(publicKey); err != nil {
		return err
	}
	utils.Outf(
		"{{green}}imported address:{{/}} %s",
		h.c.Address(publicKey),
	)
	return nil
}

func (h *Handler) SetKey(lookupBalance func(int, string, string, uint32, ids.ID) error) error {
	keys, err := h.GetKeys()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		utils.Outf("{{red}}no stored keys{{/}}\n")
		return nil
	}
	chainID, uris, err := h.GetDefaultChain()
	if err != nil {
		return err
	}
	if len(uris) == 0 {
		utils.Outf("{{red}}no available chains{{/}}\n")
		return nil
	}
	rcli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := rcli.Network(context.TODO())
	if err != nil {
		return err
	}
	utils.Outf("{{cyan}}stored keys:{{/}} %d\n", len(keys))
	for i := 0; i < len(keys); i++ {
		if err := lookupBalance(i, h.c.Address(keys[i].PublicKey()), uris[0], networkID, chainID); err != nil {
			return err
		}
	}

	// Select key
	keyIndex, err := h.PromptChoice("set default key", len(keys))
	if err != nil {
		return err
	}
	key := keys[keyIndex]
	return h.StoreDefaultKey(key.PublicKey())
}

func (h *Handler) Balance(checkAllChains bool, promptAsset bool, printBalance func(ed25519.PublicKey, string, uint32, ids.ID, ids.ID) error) error {
	priv, err := h.GetDefaultKey()
	if err != nil {
		return err
	}
	chainID, uris, err := h.GetDefaultChain()
	if err != nil {
		return err
	}
	var assetID ids.ID
	if promptAsset {
		assetID, err = h.PromptAsset("assetID", true)
		if err != nil {
			return err
		}
	}

	max := len(uris)
	if !checkAllChains {
		max = 1
	}
	for _, uri := range uris[:max] {
		utils.Outf("{{yellow}}uri:{{/}} %s\n", uri)
		rcli := rpc.NewJSONRPCClient(uris[0])
		networkID, _, _, err := rcli.Network(context.TODO())
		if err != nil {
			return err
		}
		if err := printBalance(priv.PublicKey(), uri, networkID, chainID, assetID); err != nil {
			return err
		}
	}
	return nil
}

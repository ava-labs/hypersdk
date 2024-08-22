// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"context"

	"github.com/ava-labs/hypersdk/cli/prompt"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

func (h *Handler) SetKey() error {
	keys, err := h.GetKeys()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		utils.Outf("{{red}}no stored keys{{/}}\n")
		return nil
	}
	chainID, uris, err := h.GetDefaultChain(true)
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
		addrStr := h.c.Address(keys[i].Address)
		balance, err := h.c.LookupBalance(addrStr, uris[0], networkID, chainID)
		if err != nil {
			return err
		}
		utils.Outf(
			"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
			i,
			addrStr,
			utils.FormatBalance(balance, h.c.Decimals()),
			h.c.Symbol(),
		)
	}

	// Select key
	keyIndex, err := prompt.Choice("set default key", len(keys))
	if err != nil {
		return err
	}
	key := keys[keyIndex]
	return h.StoreDefaultKey(key.Address)
}

func (h *Handler) Balance(checkAllChains bool) error {
	addr, _, err := h.GetDefaultKey(true)
	if err != nil {
		return err
	}
	chainID, uris, err := h.GetDefaultChain(true)
	if err != nil {
		return err
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
		addrStr := h.c.Address(addr)
		balance, err := h.c.LookupBalance(addrStr, uris[0], networkID, chainID)
		if err != nil {
			return err
		}
		utils.Outf(
			"{{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
			addrStr,
			utils.FormatBalance(balance, h.c.Decimals()),
			h.c.Symbol(),
		)
	}
	return nil
}

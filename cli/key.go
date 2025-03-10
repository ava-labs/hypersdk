// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"github.com/ava-labs/hypersdk/cli/prompt"
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
	_, uris, err := h.GetDefaultChain(true)
	if err != nil {
		return err
	}
	if len(uris) == 0 {
		utils.Outf("{{red}}no available chains{{/}}\n")
		return nil
	}
	utils.Outf("{{cyan}}stored keys:{{/}} %d\n", len(keys))
	for i := 0; i < len(keys); i++ {
		addrStr := keys[i].Address
		balance, err := h.c.LookupBalance(addrStr, uris[0])
		if err != nil {
			return err
		}
		utils.Outf(
			"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
			i,
			addrStr,
			utils.FormatBalance(balance),
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
	_, uris, err := h.GetDefaultChain(true)
	if err != nil {
		return err
	}

	maxURIs := len(uris)
	if !checkAllChains {
		maxURIs = 1
	}
	for _, uri := range uris[:maxURIs] {
		utils.Outf("{{yellow}}uri:{{/}} %s\n", uri)
		balance, err := h.c.LookupBalance(addr, uris[0])
		if err != nil {
			return err
		}
		utils.Outf(
			"{{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
			addr,
			utils.FormatBalance(balance),
			h.c.Symbol(),
		)
	}
	return nil
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"context"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/cli/prompt"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/throughput"
)

// BuildSpammer prompts the user for the spammer parameters. If [defaults], the default values are used once the
// chain and root key are selected. Otherwise, the user is prompted for all parameters.
func (h *Handler) BuildSpammer(sh throughput.SpamHelper, spamKey string, defaults bool) (*throughput.Spammer, error) {
	// Select chain
	chains, err := h.GetChains()
	if err != nil {
		return nil, err
	}
	_, uris, err := prompt.SelectChain("select chainID", chains)
	if err != nil {
		return nil, err
	}

	if err := sh.CreateClient(uris[0]); err != nil {
		return nil, err
	}

	var key *auth.PrivateKey

	if len(spamKey) == 0 {
		// Select root key
		keys, err := h.GetKeys()
		if err != nil {
			return nil, err
		}
		keyIndex, err := prompt.Choice("select root key", len(keys))
		if err != nil {
			return nil, err
		}
		key = keys[keyIndex]
	} else {
		key, err = auth.FromString(auth.ED25519ID, spamKey)
		if err != nil {
			return nil, err
		}
	}
	// No longer using db, so we close
	if err := h.CloseDatabase(); err != nil {
		return nil, err
	}

	if defaults {
		sc := throughput.NewDefaultConfig(uris, key)
		return throughput.NewSpammer(sc, sh)
	}
	// Collect parameters
	numAccounts, err := prompt.Int("number of accounts", consts.MaxInt)
	if err != nil {
		return nil, err
	}
	if numAccounts < 2 {
		return nil, ErrInsufficientAccounts
	}
	sZipf, err := prompt.Float("s (Zipf distribution = [(v+k)^(-s)], Default = 1.01)", consts.MaxFloat64)
	if err != nil {
		return nil, err
	}
	vZipf, err := prompt.Float("v (Zipf distribution = [(v+k)^(-s)], Default = 2.7)", consts.MaxFloat64)
	if err != nil {
		return nil, err
	}

	txsPerSecond, err := prompt.Int("txs to try and issue per second", consts.MaxInt)
	if err != nil {
		return nil, err
	}
	minTxsPerSecond, err := prompt.Int("minimum txs to issue per second", consts.MaxInt)
	if err != nil {
		return nil, err
	}
	txsPerSecondStep, err := prompt.Int("txs to increase per second", consts.MaxInt)
	if err != nil {
		return nil, err
	}
	numClients, err := prompt.Int("number of clients per node", consts.MaxInt)
	if err != nil {
		return nil, err
	}

	sc := throughput.NewConfig(
		uris,
		key,
		sZipf,
		vZipf,
		txsPerSecond,
		minTxsPerSecond,
		txsPerSecondStep,
		numClients,
		numAccounts,
	)

	return throughput.NewSpammer(sc, sh)
}

func (h *Handler) Spam(ctx context.Context, sh throughput.SpamHelper, spamKey string, defaults bool) error {
	spammer, err := h.BuildSpammer(sh, spamKey, defaults)
	if err != nil {
		return err
	}

	return spammer.Spam(ctx, sh, false, h.c.Symbol())
}

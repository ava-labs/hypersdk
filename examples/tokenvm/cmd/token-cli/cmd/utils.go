// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	hconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/manifoldco/promptui"
)

func promptAddress(label string) (crypto.PublicKey, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			_, err := utils.ParseAddress(input)
			return err
		},
	}
	recipient, err := promptText.Run()
	if err != nil {
		return crypto.EmptyPublicKey, err
	}
	recipient = strings.TrimSpace(recipient)
	return utils.ParseAddress(recipient)
}

func promptString(label string) (string, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			return nil
		},
	}
	text, err := promptText.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), err
}

func promptAsset(label string, allowNative bool) (ids.ID, error) {
	text := fmt.Sprintf("%s (use TKN for native token)", label)
	if !allowNative {
		text = label
	}
	promptText := promptui.Prompt{
		Label: text,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			if allowNative && len(input) == 3 && input == consts.Symbol {
				return nil
			}
			_, err := ids.FromString(input)
			return err
		},
	}
	asset, err := promptText.Run()
	if err != nil {
		return ids.Empty, err
	}
	asset = strings.TrimSpace(asset)
	var assetID ids.ID
	if asset != consts.Symbol {
		assetID, err = ids.FromString(asset)
		if err != nil {
			return ids.Empty, err
		}
	}
	if !allowNative && assetID == ids.Empty {
		return ids.Empty, ErrInvalidChoice
	}
	return assetID, nil
}

func promptAmount(
	label string,
	assetID ids.ID,
	balance uint64,
	f func(input uint64) error,
) (uint64, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			var amount uint64
			var err error
			if assetID == ids.Empty {
				amount, err = hutils.ParseBalance(input)
			} else {
				amount, err = strconv.ParseUint(input, 10, 64)
			}
			if err != nil {
				return err
			}
			if amount > balance {
				return ErrInsufficientBalance
			}
			if f != nil {
				return f(amount)
			}
			return nil
		},
	}
	rawAmount, err := promptText.Run()
	if err != nil {
		return 0, err
	}
	rawAmount = strings.TrimSpace(rawAmount)
	var amount uint64
	if assetID == ids.Empty {
		amount, err = hutils.ParseBalance(rawAmount)
	} else {
		amount, err = strconv.ParseUint(rawAmount, 10, 64)
	}
	return amount, err
}

func promptInt(
	label string,
) (int, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			amount, err := strconv.Atoi(input)
			if err != nil {
				return err
			}
			if amount <= 0 {
				return fmt.Errorf("%d must be > 0", amount)
			}
			return nil
		},
	}
	rawAmount, err := promptText.Run()
	if err != nil {
		return 0, err
	}
	rawAmount = strings.TrimSpace(rawAmount)
	return strconv.Atoi(rawAmount)
}

func promptChoice(label string, max int) (int, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			index, err := strconv.Atoi(input)
			if err != nil {
				return err
			}
			if index >= max || index < 0 {
				return ErrIndexOutOfRange
			}
			return nil
		},
	}
	rawIndex, err := promptText.Run()
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(rawIndex)
}

func promptTime(label string) (int64, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			_, err := strconv.ParseInt(input, 10, 64)
			return err
		},
	}
	rawTime, err := promptText.Run()
	if err != nil {
		return -1, err
	}
	return strconv.ParseInt(rawTime, 10, 64)
}

func promptContinue() (bool, error) {
	promptText := promptui.Prompt{
		Label: "continue (y/n)",
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			lower := strings.ToLower(input)
			if lower == "y" || lower == "n" {
				return nil
			}
			return ErrInvalidChoice
		},
	}
	rawContinue, err := promptText.Run()
	if err != nil {
		return false, err
	}
	cont := strings.ToLower(rawContinue)
	if cont == "n" {
		hutils.Outf("{{red}}exiting...{{/}}\n")
		return false, nil
	}
	return true, nil
}

func promptBool(label string) (bool, error) {
	promptText := promptui.Prompt{
		Label: fmt.Sprintf("%s (y/n)", label),
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			lower := strings.ToLower(input)
			if lower == "y" || lower == "n" {
				return nil
			}
			return ErrInvalidChoice
		},
	}
	rawContinue, err := promptText.Run()
	if err != nil {
		return false, err
	}
	cont := strings.ToLower(rawContinue)
	if cont == "n" {
		return false, nil
	}
	return true, nil
}

func promptID(label string) (ids.ID, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			_, err := ids.FromString(input)
			return err
		},
	}
	rawID, err := promptText.Run()
	if err != nil {
		return ids.Empty, err
	}
	rawID = strings.TrimSpace(rawID)
	id, err := ids.FromString(rawID)
	if err != nil {
		return ids.Empty, err
	}
	return id, nil
}

func promptChain(label string, excluded set.Set[ids.ID]) (ids.ID, []string, error) {
	chains, err := GetChains()
	if err != nil {
		return ids.Empty, nil, err
	}
	filteredChains := make([]ids.ID, 0, len(chains))
	excludedChains := []ids.ID{}
	for chainID := range chains {
		if excluded != nil && excluded.Contains(chainID) {
			excludedChains = append(excludedChains, chainID)
			continue
		}
		filteredChains = append(filteredChains, chainID)
	}
	if len(filteredChains) == 0 {
		return ids.Empty, nil, ErrNoChains
	}

	// Select chain
	hutils.Outf(
		"{{cyan}}available chains:{{/}} %d {{cyan}}excluded:{{/}} %+v\n",
		len(filteredChains),
		excludedChains,
	)
	keys := make([]ids.ID, 0, len(filteredChains))
	for _, chainID := range filteredChains {
		hutils.Outf(
			"%d) {{cyan}}chainID:{{/}} %s\n",
			len(keys),
			chainID,
		)
		keys = append(keys, chainID)
	}

	chainIndex, err := promptChoice(label, len(keys))
	if err != nil {
		return ids.Empty, nil, err
	}
	chainID := keys[chainIndex]
	return chainID, chains[chainID], nil
}

func valueString(assetID ids.ID, value uint64) string {
	if assetID == ids.Empty {
		return hutils.FormatBalance(value)
	}
	// Custom assets are denoted in raw units
	return strconv.FormatUint(value, 10)
}

func assetString(assetID ids.ID) string {
	if assetID == ids.Empty {
		return consts.Symbol
	}
	return assetID.String()
}

func printStatus(txID ids.ID, success bool) {
	status := "⚠️"
	if success {
		status = "✅"
	}
	hutils.Outf("%s {{yellow}}txID:{{/}} %s\n", status, txID)
}

func getAssetInfo(
	ctx context.Context,
	cli *trpc.JSONRPCClient,
	publicKey crypto.PublicKey,
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
		valueString(assetID, balance),
		assetString(assetID),
	)
	return balance, sourceChainID, nil
}

func defaultActor() (ids.ID, crypto.PrivateKey, *auth.ED25519Factory, *rpc.JSONRPCClient, *trpc.JSONRPCClient, error) {
	priv, err := GetDefaultKey()
	if err != nil {
		return ids.Empty, crypto.EmptyPrivateKey, nil, nil, nil, err
	}
	chainID, uris, err := GetDefaultChain()
	if err != nil {
		return ids.Empty, crypto.EmptyPrivateKey, nil, nil, nil, err
	}
	// For [defaultActor], we always send requests to the first returned URI.
	return chainID, priv, auth.NewED25519Factory(
			priv,
		), rpc.NewJSONRPCClient(
			uris[0],
		), trpc.NewJSONRPCClient(
			uris[0],
			chainID,
		), nil
}

func GetDefaultKey() (crypto.PrivateKey, error) {
	v, err := GetDefault(defaultKeyKey)
	if err != nil {
		return crypto.EmptyPrivateKey, err
	}
	if len(v) == 0 {
		return crypto.EmptyPrivateKey, ErrNoKeys
	}
	publicKey := crypto.PublicKey(v)
	priv, err := GetKey(publicKey)
	if err != nil {
		return crypto.EmptyPrivateKey, err
	}
	hutils.Outf("{{yellow}}address:{{/}} %s\n", utils.Address(publicKey))
	return priv, nil
}

func GetDefaultChain() (ids.ID, []string, error) {
	v, err := GetDefault(defaultChainKey)
	if err != nil {
		return ids.Empty, nil, err
	}
	if len(v) == 0 {
		return ids.Empty, nil, ErrNoChains
	}
	chainID := ids.ID(v)
	uris, err := GetChain(chainID)
	if err != nil {
		return ids.Empty, nil, err
	}
	hutils.Outf("{{yellow}}chainID:{{/}} %s\n", chainID)
	return chainID, uris, nil
}

func CloseDatabase() error {
	if db == nil {
		return nil
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("unable to close database: %w", err)
	}
	return nil
}

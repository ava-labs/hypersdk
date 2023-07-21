// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/basevm/auth"
	"github.com/ava-labs/hypersdk/examples/basevm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/basevm/rpc"
	"github.com/ava-labs/hypersdk/examples/basevm/utils"
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

func promptAmount(
	label string,
	balance uint64,
	f func(input uint64) error,
) (uint64, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			amount, err := hutils.ParseBalance(input)
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
	return hutils.ParseBalance(rawAmount)
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

func promptChain(label string) (ids.ID, []string, error) {
	chains, err := GetChains()
	if err != nil {
		return ids.Empty, nil, err
	}
	filteredChains := make([]ids.ID, 0, len(chains))
	for chainID := range chains {
		filteredChains = append(filteredChains, chainID)
	}
	if len(filteredChains) == 0 {
		return ids.Empty, nil, ErrNoChains
	}

	// Select chain
	hutils.Outf(
		"{{cyan}}available chains:{{/}} %d\n",
		len(filteredChains),
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

func printStatus(txID ids.ID, success bool) {
	status := "⚠️"
	if success {
		status = "✅"
	}
	hutils.Outf("%s {{yellow}}txID:{{/}} %s\n", status, txID)
}

func getBalance(
	ctx context.Context,
	cli *trpc.JSONRPCClient,
	publicKey crypto.PublicKey,
) (uint64, error) {
	addr := utils.Address(publicKey)
	balance, err := cli.Balance(ctx, addr)
	if err != nil {
		return 0, err
	}
	if balance == 0 {
		hutils.Outf("{{red}}balance:{{/}} 0 %s\n", consts.Symbol)
		hutils.Outf("{{red}}please send funds to %s{{/}}\n", addr)
		hutils.Outf("{{red}}exiting...{{/}}\n")
		return 0, nil
	}
	hutils.Outf(
		"{{yellow}}balance:{{/}} %s %s\n",
		hutils.FormatBalance(balance),
		consts.Symbol,
	)
	return balance, nil
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

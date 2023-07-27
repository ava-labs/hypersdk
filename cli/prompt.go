// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/manifoldco/promptui"
)

func (h *Handler) PromptAddress(label string) (crypto.PublicKey, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			_, err := h.c.ParseAddress(input)
			return err
		},
	}
	recipient, err := promptText.Run()
	if err != nil {
		return crypto.EmptyPublicKey, err
	}
	recipient = strings.TrimSpace(recipient)
	return h.c.ParseAddress(recipient)
}

func (*Handler) PromptString(label string, min int, max int) (string, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) < min {
				return ErrInputEmpty
			}
			if len(input) > max {
				return ErrInputTooLarge
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

func (h *Handler) PromptAsset(label string, allowNative bool) (ids.ID, error) {
	symbol := h.c.Symbol()
	text := fmt.Sprintf("%s (use %s for native token)", label, symbol)
	if !allowNative {
		text = label
	}
	promptText := promptui.Prompt{
		Label: text,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			if allowNative && input == symbol {
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
	if asset != symbol {
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

func (*Handler) PromptAmount(
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
				amount, err = utils.ParseBalance(input)
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
		amount, err = utils.ParseBalance(rawAmount)
	} else {
		amount, err = strconv.ParseUint(rawAmount, 10, 64)
	}
	return amount, err
}

func (*Handler) PromptInt(
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

func (*Handler) PromptChoice(label string, max int) (int, error) {
	// TODO: auto-select if only 1 option provided
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

func (*Handler) PromptTime(label string) (int64, error) {
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

func (*Handler) PromptContinue() (bool, error) {
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
		utils.Outf("{{red}}exiting...{{/}}\n")
		return false, nil
	}
	return true, nil
}

func (*Handler) PromptBool(label string) (bool, error) {
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

func (*Handler) PromptID(label string) (ids.ID, error) {
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

func (h *Handler) PromptChain(label string, excluded set.Set[ids.ID]) (ids.ID, []string, error) {
	chains, err := h.GetChains()
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
	utils.Outf(
		"{{cyan}}available chains:{{/}} %d {{cyan}}excluded:{{/}} %+v\n",
		len(filteredChains),
		excludedChains,
	)
	keys := make([]ids.ID, 0, len(filteredChains))
	for _, chainID := range filteredChains {
		utils.Outf(
			"%d) {{cyan}}chainID:{{/}} %s\n",
			len(keys),
			chainID,
		)
		keys = append(keys, chainID)
	}

	chainIndex, err := h.PromptChoice(label, len(keys))
	if err != nil {
		return ids.Empty, nil, err
	}
	chainID := keys[chainIndex]
	return chainID, chains[chainID], nil
}

func (*Handler) ValueString(assetID ids.ID, value uint64) string {
	if assetID == ids.Empty {
		return utils.FormatBalance(value)
	}
	// Custom assets are denoted in raw units
	return strconv.FormatUint(value, 10)
}

func (h *Handler) AssetString(assetID ids.ID) string {
	if assetID == ids.Empty {
		return h.c.Symbol()
	}
	return assetID.String()
}

func (*Handler) PrintStatus(txID ids.ID, success bool) {
	status := "⚠️"
	if success {
		status = "✅"
	}
	utils.Outf("%s {{yellow}}txID:{{/}} %s\n", status, txID)
}

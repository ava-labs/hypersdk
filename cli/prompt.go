// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/manifoldco/promptui"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/utils"
)

func (h *Handler) PromptAddress(label string) (codec.Address, error) {
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
		return codec.EmptyAddress, err
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
	decimals uint8,
	balance uint64,
	f func(input uint64) error,
) (uint64, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			amount, err := utils.ParseBalance(input, decimals)
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
	return utils.ParseBalance(rawAmount, decimals)
}

func (*Handler) PromptInt(
	label string,
	max int,
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
			if amount > max {
				return fmt.Errorf("%d must be <= %d", amount, max)
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
	if max == 1 {
		utils.Outf("{{yellow}}%s:{{/}} 0 [auto-selected]\n", label)
		return 0, nil
	}
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
		Label: label + " (y/n)",
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

func (*Handler) PrintStatus(txID ids.ID, success bool) {
	status := "❌"
	if success {
		status = "✅"
	}
	utils.Outf("%s {{yellow}}txID:{{/}} %s\n", status, txID)
}

func PrintUnitPrices(d fees.Dimensions) {
	utils.Outf(
		"{{cyan}}unit prices{{/}} {{yellow}}bandwidth:{{/}} %d {{yellow}}compute:{{/}} %d {{yellow}}storage(read):{{/}} %d {{yellow}}storage(allocate):{{/}} %d {{yellow}}storage(write):{{/}} %d\n",
		d[fees.Bandwidth],
		d[fees.Compute],
		d[fees.StorageRead],
		d[fees.StorageAllocate],
		d[fees.StorageWrite],
	)
}

func ParseDimensions(d fees.Dimensions) string {
	return fmt.Sprintf(
		"bandwidth=%d compute=%d storage(read)=%d storage(allocate)=%d storage(write)=%d",
		d[fees.Bandwidth],
		d[fees.Compute],
		d[fees.StorageRead],
		d[fees.StorageAllocate],
		d[fees.StorageWrite],
	)
}

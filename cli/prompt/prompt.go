// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package prompt

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/manifoldco/promptui"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	ErrInputEmpty           = errors.New("input is empty")
	ErrInputTooLarge        = errors.New("input is too large")
	ErrInvalidChoice        = errors.New("invalid choice")
	ErrIndexOutOfRange      = errors.New("index out-of-range")
	ErrInsufficientBalance  = errors.New("insufficient balance")
	ErrDuplicate            = errors.New("duplicate")
	ErrNoChains             = errors.New("no available chains")
	ErrNoKeys               = errors.New("no available keys")
	ErrTxFailed             = errors.New("tx failed on-chain")
	ErrInsufficientAccounts = errors.New("insufficient accounts")
)

func Bytes(label string) ([]byte, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			_, err := codec.LoadHex(input, -1)
			return err
		},
	}
	hexString, err := promptText.Run()
	if err != nil {
		return nil, err
	}
	return codec.LoadHex(hexString, -1)
}

func Address(label string) (codec.Address, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			_, err := codec.StringToAddress(strings.TrimSpace(input))
			return err
		},
	}
	recipient, err := promptText.Run()
	if err != nil {
		return codec.EmptyAddress, err
	}
	recipient = strings.TrimSpace(recipient)
	return codec.StringToAddress(recipient)
}

func String(label string, minLen int, maxLen int) (string, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) < minLen {
				return ErrInputEmpty
			}
			if len(input) > maxLen {
				return ErrInputTooLarge
			}
			return nil
		},
	}
	text, err := promptText.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func Asset(label string, symbol string, allowNative bool) (ids.ID, error) {
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

func Amount(
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
			amount, err := utils.ParseBalance(input)
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
	return utils.ParseBalance(rawAmount)
}

func Int(
	label string,
	maxValue int,
) (int, error) {
	stringToInt := func(input string, maxValue int) (int, error) {
		input = strings.TrimSpace(input)

		if len(input) == 0 {
			return 0, ErrInputEmpty
		}
		amount, err := strconv.Atoi(input)
		if err != nil {
			return 0, err
		}
		if amount <= 0 {
			return 0, fmt.Errorf("%d must be > 0", amount)
		}
		if amount > maxValue {
			return 0, fmt.Errorf("%d must be <= %d", amount, maxValue)
		}
		return amount, nil
	}

	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			_, err := stringToInt(input, maxValue)
			return err
		},
	}
	rawAmount, err := promptText.Run()
	if err != nil {
		return 0, err
	}
	return stringToInt(rawAmount, maxValue)
}

func Uint(
	label string,
	maxValue uint,
) (uint, error) {
	stringToUint := func(input string, maxValue uint) (uint, error) {
		input = strings.TrimSpace(input)

		if len(input) == 0 {
			return 0, ErrInputEmpty
		}
		amount, err := strconv.ParseUint(input, 10, 0)
		if err != nil {
			return 0, err
		}
		if amount > math.MaxUint {
			return 0, fmt.Errorf("%d exceeds the maximum value for uint", amount)
		}
		if uint(amount) > maxValue {
			return 0, fmt.Errorf("%d must be <= %d", amount, maxValue)
		}
		return uint(amount), nil
	}

	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			_, err := stringToUint(input, maxValue)
			return err
		},
	}
	rawAmount, err := promptText.Run()
	if err != nil {
		return 0, err
	}
	return stringToUint(rawAmount, maxValue)
}

func Float(
	label string,
	maxValue float64,
) (float64, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return ErrInputEmpty
			}
			amount, err := strconv.ParseFloat(input, 64)
			if err != nil {
				return err
			}
			if amount <= 0 {
				return fmt.Errorf("%f must be > 0", amount)
			}
			if amount > maxValue {
				return fmt.Errorf("%f must be <= %f", amount, maxValue)
			}
			return nil
		},
	}
	rawAmount, err := promptText.Run()
	if err != nil {
		return 0, err
	}
	rawAmount = strings.TrimSpace(rawAmount)
	return strconv.ParseFloat(rawAmount, 64)
}

func Choice(label string, maxChoice int) (int, error) {
	if maxChoice == 1 {
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
			if index >= maxChoice || index < 0 {
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

func Time(label string) (int64, error) {
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

func Continue() (bool, error) {
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

func Bool(label string) (bool, error) {
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

func ID(label string) (ids.ID, error) {
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

func SelectChain(label string, chainToRPCs map[ids.ID][]string) (ids.ID, []string, error) {
	// Select chain
	utils.Outf(
		"{{cyan}}available chains:{{/}} %d\n",
		len(chainToRPCs),
	)
	keys := make([]ids.ID, 0, len(chainToRPCs))
	for chainID := range chainToRPCs {
		utils.Outf(
			"%d) {{cyan}}chainID:{{/}} %s\n",
			len(keys),
			chainID,
		)
		keys = append(keys, chainID)
	}

	chainIndex, err := Choice(label, len(keys))
	if err != nil {
		return ids.Empty, nil, err
	}
	chainID := keys[chainIndex]
	return chainID, chainToRPCs[chainID], nil
}

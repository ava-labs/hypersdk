package cmd

import (
	"errors"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/manifoldco/promptui"
)

func promptAddress(label string) (crypto.PublicKey, error) {
	promptText := promptui.Prompt{
		Label: label,
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			_, err := utils.ParseAddress(input)
			return err
		},
	}
	recipient, err := promptText.Run()
	if err != nil {
		return crypto.EmptyPublicKey, err
	}
	return utils.ParseAddress(recipient)
}

func promptAsset() (ids.ID, error) {
	promptText := promptui.Prompt{
		Label: "assetID (use TKN for native token)",
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			if len(input) == 3 && input == consts.Symbol {
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
	var assetID ids.ID
	if asset != consts.Symbol {
		assetID, err = ids.FromString(asset)
		if err != nil {
			return ids.Empty, err
		}
	}
	return assetID, nil
}

func promptAmount(assetID ids.ID, balance uint64) (uint64, error) {
	promptText := promptui.Prompt{
		Label: "amount",
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
			return nil
		},
	}
	rawAmount, err := promptText.Run()
	if err != nil {
		return 0, err
	}
	var amount uint64
	if assetID == ids.Empty {
		amount, err = hutils.ParseBalance(rawAmount)
	} else {
		amount, err = strconv.ParseUint(rawAmount, 10, 64)
	}
	return amount, err
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

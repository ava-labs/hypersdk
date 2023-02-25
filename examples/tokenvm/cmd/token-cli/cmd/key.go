// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"errors"
	"os"

	"github.com/ava-labs/hypersdk/crypto"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

var keyCmd = &cobra.Command{
	Use:   "key [options]",
	Short: "Creates a new key in the default location",
	RunE:  keyFunc,
}

func keyFunc(*cobra.Command, []string) error {
	if _, err := os.Stat(privateKeyFile); err == nil {
		// Already found, remind the user they have it
		priv, err := crypto.LoadKey(privateKeyFile)
		if err != nil {
			return err
		}
		color.Green(
			"ABORTING!!! key for %s already exists at %s",
			utils.Address(priv.PublicKey()),
			privateKeyFile,
		)
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Generate new key and save to disk
	// TODO: encrypt key
	priv, err := crypto.GeneratePrivateKey()
	if err != nil {
		return err
	}
	if err := priv.Save(privateKeyFile); err != nil {
		return err
	}
	color.Green(
		"created address %s and saved to %s",
		utils.Address(priv.PublicKey()),
		privateKeyFile,
	)
	return nil
}

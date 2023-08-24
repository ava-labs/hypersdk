// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "manage key",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var genKeyCmd = &cobra.Command{
	Use: "generate",
	RunE: func(*cobra.Command, []string) error {
		priv, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return err
		}

		utils.Outf("{{green}}created new private key with public address:{{/}} %s\n", address(priv.PublicKey()))
		return setKey(db, priv)
	},
}

func setKey(db database.Database, privateKey ed25519.PrivateKey) error {
	publicKey := privateKey.PublicKey()
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], publicKey[:])
	has, err := db.Has(k)
	if err != nil {
		return err
	}
	if has {
		return cli.ErrDuplicate
	}
	return db.Put(k, privateKey[:])
}

func getKey(db database.Database, publicKey ed25519.PublicKey) (ed25519.PrivateKey, error) {
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], publicKey[:])
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return ed25519.EmptyPrivateKey, nil
	}
	if err != nil {
		return ed25519.EmptyPrivateKey, err
	}
	return ed25519.PrivateKey(v), nil
}

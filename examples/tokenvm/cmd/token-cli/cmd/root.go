// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "token-cli" implements tokenvm client operation interface.
package cmd

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

const (
	requestTimeout = 30 * time.Second
	fsModeWrite    = 0o600
	databaseFolder = ".token-cli"
)

var (
	db      database.Database
	workDir string

	genesisFile  string
	minUnitPrice int64

	rootCmd = &cobra.Command{
		Use:        "token-cli",
		Short:      "TokenVM CLI",
		SuggestFor: []string{"token-cli", "tokencli"},
	}
)

func init() {
	p, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	workDir = p
	dbPath := filepath.Join(workDir, databaseFolder)
	db, err = pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		panic(err)
	}

	cobra.EnablePrefixMatching = true
	rootCmd.AddCommand(
		genesisCmd,
		keyCmd,
		chainCmd,

		transferCmd,

		// importAssetCmd,
		// exportAssetCmd,

		createAssetCmd,
		mintAssetCmd,
		// burnAssetCmd,
		// modifyAssetCmd,

		createOrderCmd,
		fillOrderCmd,
		closeOrderCmd,
	)

	// genesis
	genesisCmd.AddCommand(
		genGenesisCmd,
	)
	genGenesisCmd.PersistentFlags().StringVar(
		&genesisFile,
		"genesis-file",
		filepath.Join(workDir, "genesis.json"),
		"genesis file path",
	)
	genGenesisCmd.PersistentFlags().Int64Var(
		&minUnitPrice,
		"min-unit-price",
		-1,
		"minimum price",
	)

	// key
	keyCmd.AddCommand(
		genKeyCmd,
		importKeyCmd,
		setKeyCmd,
		balanceKeyCmd,
	)

	// chain
	chainCmd.AddCommand(
		importChainCmd,
		setChainCmd,
		chainInfoCmd,
		watchChainCmd,
	)
}

func GetDefaultKey() (crypto.PrivateKey, error) {
	v, err := GetDefault(defaultKeyKey)
	if err != nil {
		return crypto.EmptyPrivateKey, err
	}
	if len(v) == 0 {
		utils.Outf("{{red}}no available keys{{/}}\n")
		return crypto.EmptyPrivateKey, nil
	}
	return crypto.PrivateKey(v), nil
}

func GetDefaultChain() (string, error) {
	v, err := GetDefault(defaultChainKey)
	if err != nil {
		return "", err
	}
	if len(v) == 0 {
		utils.Outf("{{red}}no available chains{{/}}\n")
		return "", nil
	}
	return string(v), nil
}

func Execute() error {
	defer db.Close()
	return rootCmd.Execute()
}

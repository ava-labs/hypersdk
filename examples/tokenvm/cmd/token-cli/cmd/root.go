// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "token-cli" implements tokenvm client operation interface.
package cmd

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	tutils "github.com/ava-labs/hypersdk/examples/tokenvm/utils"
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
		actionCmd,

		fillOrderCmd,
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

	// actions
	actionCmd.AddCommand(
		transferCmd,

		// importAssetCmd,
		// exportAssetCmd,

		createAssetCmd,
		mintAssetCmd,
		// burnAssetCmd,
		// modifyAssetCmd,

		createOrderCmd,
		closeOrderCmd,
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
	publicKey := crypto.PublicKey(v)
	priv, err := GetKey(publicKey)
	if err != nil {
		return crypto.EmptyPrivateKey, err
	}
	utils.Outf("{{yellow}}address:{{/}} %s\n", tutils.Address(publicKey))
	return priv, nil
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
	chainID := ids.ID(v)
	uri, err := GetChain(chainID)
	if err != nil {
		return "", err
	}
	utils.Outf("{{yellow}}chainID:{{/}} %s {{yellow}}uri:{{/}} %s\n", chainID, uri)
	return uri, nil
}

func Execute() error {
	defer db.Close()
	return rootCmd.Execute()
}

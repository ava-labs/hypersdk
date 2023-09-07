// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/examples"
	xutils "github.com/ava-labs/hypersdk/x/programs/utils"
)

const (
	defaultDatabase = ".simulator"
)

func init() {
	cobra.EnablePrefixMatching = true
	rootCmd.AddCommand(
		programCmd,
		keyCmd,
	)

	programCmd.AddCommand(
		programCreateCmd,
		programInvokeCmd,
	)

	keyCmd.AddCommand(
		genKeyCmd,
	)

	rootCmd.PersistentFlags().StringVar(
		&dbPath,
		"database",
		defaultDatabase,
		"path to database (will create if missing)",
	)

	rootCmd.PersistentFlags().StringVar(
		&callerAddress,
		"caller",
		"",
		"address of caller",
	)

	rootCmd.PersistentPreRunE = func(*cobra.Command, []string) (err error) {
		log = xutils.NewLoggerWithLogLevel(logging.Debug)
		db, _, err = pebble.New(dbPath, pebble.NewDefaultConfig())
		if err != nil {
			return err
		}
		utils.Outf("{{yellow}}database:{{/}} %s\n", dbPath)
		return nil
	}

	rootCmd.PersistentPostRunE = func(*cobra.Command, []string) error {
		return db.Close()
	}
	programInvokeCmd.PersistentFlags().Uint64Var(
		&programID,
		"id",
		0,
		"id of the program",
	)

	programInvokeCmd.PersistentFlags().StringVar(
		&functionName,
		"function",
		"",
		"name of the function to invoke",
	)

	programInvokeCmd.PersistentFlags().StringVar(
		&params,
		"params",
		"",
		"comma separated list of params to pass to the function",
	)

	programCreateCmd.PersistentFlags().Uint64Var(
		&maxFee,
		"max-fee",
		examples.DefaultMaxFee,
		"max fee to pay for the action",
	)

	programInvokeCmd.PersistentFlags().Uint64Var(
		&maxFee,
		"max-fee",
		examples.DefaultMaxFee,
		"max fee to pay for the action",
	)
}

var (
	callerAddress string
	programID     uint64
	functionName  string
	dbPath        string
	params        string
	maxFee        uint64
	db            database.Database
	log           logging.Logger
	rootCmd       = &cobra.Command{
		Use:   "simulator",
		Short: "HyperSDK program simulator",
	}
)

func Execute() error {
	return rootCmd.Execute()
}

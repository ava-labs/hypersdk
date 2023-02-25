// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
)

var (
	genesisFile string

	minUnitPrice int64
)

func init() {
	genesisCmd.PersistentFlags().StringVar(
		&genesisFile,
		"genesis-file",
		filepath.Join(workDir, "genesis.json"),
		"genesis file path",
	)
	genesisCmd.PersistentFlags().Int64Var(
		&minUnitPrice,
		"min-unit-price",
		-1,
		"minimum price",
	)
}

var genesisCmd = &cobra.Command{
	Use:   "genesis [custom allocations file] [options]",
	Short: "Creates a new genesis in the default location",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("invalid args")
		}
		return nil
	},
	RunE: genesisFunc,
}

func genesisFunc(_ *cobra.Command, args []string) error {
	g := genesis.Default()
	if minUnitPrice >= 0 {
		g.MinUnitPrice = uint64(minUnitPrice)
	}

	a, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	allocs := []*genesis.CustomAllocation{}
	if err := json.Unmarshal(a, &allocs); err != nil {
		return err
	}
	g.CustomAllocation = allocs

	b, err := json.Marshal(g)
	if err != nil {
		return err
	}
	if err := os.WriteFile(genesisFile, b, fsModeWrite); err != nil {
		return err
	}
	color.Green("created genesis and saved to %s", genesisFile)
	return nil
}

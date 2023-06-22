// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
)

var genesisCmd = &cobra.Command{
	Use: "genesis",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var genGenesisCmd = &cobra.Command{
	Use:   "generate [custom allocations file] [options]",
	Short: "Creates a new genesis in the default location",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		g := genesis.Default()
		if minUnitPrice >= 0 {
			g.MinUnitPrice = uint64(minUnitPrice)
		}
		if maxBlockUnits >= 0 {
			g.MaxBlockUnits = uint64(maxBlockUnits)
		}
		if windowTargetUnits >= 0 {
			g.WindowTargetUnits = uint64(windowTargetUnits)
		}
		if windowTargetBlocks >= 0 {
			g.WindowTargetBlocks = uint64(windowTargetBlocks)
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
	},
}

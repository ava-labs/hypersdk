// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
)

var genesisCmd = &cobra.Command{
	Use: "genesis",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var genGenesisCmd = &cobra.Command{
	Use:   "generate [custom allocates file] [options]",
	Short: "Creates a new genesis in the default location",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		g := genesis.Default()
		if len(minUnitPrice) > 0 {
			d, err := chain.ParseDimensions(minUnitPrice)
			if err != nil {
				return err
			}
			g.MinUnitPrice = d
		}
		if len(maxChunkUnits) > 0 {
			d, err := chain.ParseDimensions(maxChunkUnits)
			if err != nil {
				return err
			}
			g.MaxChunkUnits = d
		}
		if minBlockGap >= 0 {
			g.MinBlockGap = minBlockGap
		}
		if epochDuration >= 0 {
			g.EpochDuration = epochDuration
		}
		if validityWindow >= 0 {
			g.ValidityWindow = validityWindow
		}
		if g.EpochDuration < g.ValidityWindow {
			return fmt.Errorf("epoch duration (%d) must be greater than or equal to validity window (%d)", g.EpochDuration, g.ValidityWindow)
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

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/vm"
)

var rulesCmd = &cobra.Command{
	Use: "rules",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var genRulesCmd = &cobra.Command{
	Use:   "generate [options]",
	Short: "Creates a new rules in the default location",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		g := vm.DefaultRules()
		if len(minUnitPrice) > 0 {
			d, err := fees.ParseDimensions(minUnitPrice)
			if err != nil {
				return err
			}
			g.MinUnitPrice = d
		}
		if len(maxBlockUnits) > 0 {
			d, err := fees.ParseDimensions(maxBlockUnits)
			if err != nil {
				return err
			}
			g.MaxBlockUnits = d
		}
		if len(windowTargetUnits) > 0 {
			d, err := fees.ParseDimensions(windowTargetUnits)
			if err != nil {
				return err
			}
			g.WindowTargetUnits = d
		}
		if minBlockGap >= 0 {
			g.MinBlockGap = minBlockGap
		}

		b, err := json.Marshal(g)
		if err != nil {
			return err
		}
		if err := os.WriteFile(rulesFile, b, fsModeWrite); err != nil {
			return err
		}
		color.Green("created rules and saved to %s", rulesFile)
		return nil
	},
}

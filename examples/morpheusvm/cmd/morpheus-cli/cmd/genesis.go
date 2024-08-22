// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/vm"
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
		combined := &vm.GenesisWithInitialRules[*vm.AllocationGenesis, *vm.Rules]{}
		combined.Genesis = vm.NewAllocationGenesis(
			func(saddr string) (codec.Address, error) {
				return codec.ParseAddressBech32(consts.HRP, saddr)
			},
			storage.SetBalance)
		combined.InitialRules = vm.NewRules()

		if len(minUnitPrice) > 0 {
			d, err := fees.ParseDimensions(minUnitPrice)
			if err != nil {
				return err
			}
			combined.InitialRules.MinUnitPrice = d
		}
		if len(maxBlockUnits) > 0 {
			d, err := fees.ParseDimensions(maxBlockUnits)
			if err != nil {
				return err
			}
			combined.InitialRules.MaxBlockUnits = d
		}
		if len(windowTargetUnits) > 0 {
			d, err := fees.ParseDimensions(windowTargetUnits)
			if err != nil {
				return err
			}
			combined.InitialRules.WindowTargetUnits = d
		}
		if minBlockGap >= 0 {
			combined.InitialRules.MinBlockGap = minBlockGap
		}

		a, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		var allocs []*vm.CustomAllocation
		if err := json.Unmarshal(a, &allocs); err != nil {
			return err
		}
		combined.Genesis.CustomAllocation = allocs

		b, err := json.Marshal(combined)
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

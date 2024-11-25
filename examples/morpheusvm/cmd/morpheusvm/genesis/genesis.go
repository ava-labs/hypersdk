// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
)

const minBlockGap = 100

func init() {
	cobra.EnablePrefixMatching = true
}

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "genesis",
		Short: "Prints out the genesis",
		RunE:  genesisFunc,
	}
	return cmd
}

func genesisFunc(*cobra.Command, []string) error {
	networkConfig, err := workload.NewTestNetworkConfig(minBlockGap)
	if err != nil {
		return err
	}

	fmt.Println(networkConfig.GenesisBytes())
	return nil
}

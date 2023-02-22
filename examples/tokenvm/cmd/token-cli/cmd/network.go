// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
)

var networkCmd = &cobra.Command{
	Use:   "network [options]",
	Short: "View information about this instance of the tokenvm",
	RunE:  networkFunc,
}

func networkFunc(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("expected exactly 0 arguments, got %d", len(args))
	}
	cli := client.New(uri)
	networkID, subnetID, chainID, err := cli.Network(context.Background())
	if err != nil {
		return err
	}
	color.Cyan("networkID=%d subnetID=%s chainID=%s", networkID, subnetID, chainID)
	return nil
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/ulimit"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/nftvm/cmd/nftvm/version"
	"github.com/ava-labs/hypersdk/examples/nftvm/controller"
)

var rootCmd = &cobra.Command{
	Use:        "nftvm",
	Short:      "BaseVM agent",
	SuggestFor: []string{"nftvm"},
	RunE:       runFunc,
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "nftvm failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runFunc(*cobra.Command, []string) error {
	if err := ulimit.Set(ulimit.DefaultFDLimit, logging.NoLog{}); err != nil {
		return fmt.Errorf("%w: failed to set fd limit correctly", err)
	}
	return rpcchainvm.Serve(context.TODO(), controller.New())
}

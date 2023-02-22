// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/vms/rpcchainvm"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/indexvm/version"
	"github.com/ava-labs/hypersdk/examples/tokenvm/controller"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "indexvm",
	Short:      "IndexVM agent",
	SuggestFor: []string{"indexvm"},
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
		fmt.Fprintf(os.Stderr, "indexvm failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runFunc(*cobra.Command, []string) error {
	rpcchainvm.Serve(controller.New())
	return nil
}

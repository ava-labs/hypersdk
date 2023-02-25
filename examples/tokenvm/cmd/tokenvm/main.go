// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/vms/rpcchainvm"
	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/tokenvm/version"
	"github.com/ava-labs/hypersdk/examples/tokenvm/controller"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "tokenvm",
	Short:      "TokenVM agent",
	SuggestFor: []string{"tokenvm"},
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
		fmt.Fprintf(os.Stderr, "tokenvm failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runFunc(*cobra.Command, []string) error {
	rpcchainvm.Serve(controller.New())
	return nil
}

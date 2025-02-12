// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/evm/cmd/evm/version"
	"github.com/ava-labs/hypersdk/snow"

	evm "github.com/ava-labs/hypersdk/examples/evm/vm"
)

var rootCmd = &cobra.Command{
	Use:        "evm",
	Short:      "BaseVM agent",
	SuggestFor: []string{"evm"},
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
		fmt.Fprintf(os.Stderr, "evm failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runFunc(*cobra.Command, []string) error {
	if err := ulimit.Set(ulimit.DefaultFDLimit, logging.NoLog{}); err != nil {
		return fmt.Errorf("%w: failed to set fd limit correctly", err)
	}

	v, err := evm.New()
	if err != nil {
		return err
	}

	return rpcchainvm.Serve(context.TODO(), snow.NewSnowVM[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]("v0.0.1", v))
}

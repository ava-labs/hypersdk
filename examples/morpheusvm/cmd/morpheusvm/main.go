// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/ulimit"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/cmd/morpheusvm/version"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/controller"
)

var rootCmd = &cobra.Command{
	Use:        "morpheusvm",
	Short:      "BaseVM agent",
	SuggestFor: []string{"morpheusvm"},
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
		fmt.Fprintf(os.Stderr, "morpheusvm failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runFunc(*cobra.Command, []string) error {
	logFactory := logging.NewFactory(logging.Config{
		DisplayLevel: logging.Debug,
	})

	log, err := logFactory.Make("main")
	if err != nil {
		return fmt.Errorf("failed to initialize log: %w", err)
	}

	if err := ulimit.Set(ulimit.DefaultFDLimit, log); err != nil {
		return fmt.Errorf("%w: failed to set fd limit correctly", err)
	}

	// TODO use disk-based db
	controller, err := controller.New(log, trace.Noop, memdb.New())
	if err != nil {
		return fmt.Errorf("failed to initialize controller: %w", err)
	}

	return rpcchainvm.Serve(context.TODO(), controller)
}

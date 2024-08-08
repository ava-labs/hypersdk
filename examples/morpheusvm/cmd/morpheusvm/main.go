// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
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

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/cmd/morpheusvm/version"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/controller"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/vm"

	lconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
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

	txDBIndexer := indexer.NewTxDBIndexer(memdb.New())
	indexerFactory := indexer.NewSubscriptionFactory(txDBIndexer)
	indexerAPI := indexer.NewAPIFactory(txDBIndexer, "morpheusvm", "/indexer")

	server, handler := rpc.NewWebSocketServer(
		log,
		trace.Noop,
		lconsts.ActionRegistry,
		lconsts.AuthRegistry,
		10000000,
	)

	webSocketFactory := rpc.NewPubSubFactory(handler)
	removeTxSubscription := rpc.SubscriptionFuncFactory[vm.TxRemovedEvent]{
		AcceptF: func(event vm.TxRemovedEvent) error {
			return server.RemoveTx(event.TxID, event.Err)
		},
	}

	acceptBlockSubscription := &rpc.SubscriptionFuncFactory[*chain.StatelessBlock]{
		AcceptF: func(event *chain.StatelessBlock) error {
			return server.AcceptBlock(event)
		},
	}

	controller, err := controller.New(
		vm.WithBlockSubscriptions[*controller.Controller](indexerFactory, acceptBlockSubscription),
		vm.WithRemoveTxSubscriptions[*controller.Controller](removeTxSubscription),
		vm.WithVMAPIs[*controller.Controller](
			rpc.JSONRPCServerFactory{},
			indexerAPI,
			webSocketFactory,
		),
		vm.WithControllerAPIs[*controller.Controller](&lrpc.JSONRPCServerFactory{}),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize controller: %w", err)
	}

	return rpcchainvm.Serve(context.TODO(), controller)
}

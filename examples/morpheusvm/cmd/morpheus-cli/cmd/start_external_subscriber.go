// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/controller"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
)

var (
	logFactory          logging.Factory
	log                 logging.Logger
	acceptedSubscribers []event.Subscription[externalsubscriber.ExternalSubscriberSubscriptionData]
)

// Used as a lambda function for creating ExternalSubscriberServer parser
func ParserFactory(networkID uint32, chainID ids.ID, genesisBytes []byte) (chain.Parser, error) {
	var genesis genesis.Genesis
	if err := json.Unmarshal(genesisBytes, &genesis); err != nil {
		return nil, err
	}
	parser := controller.NewParser(networkID, chainID, &genesis)
	return parser, nil
}

var startExternalSubscriberCommand = &cobra.Command{
	Use:   "start-external-subscriber",
	Short: "Start an external subscriber server",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize global variables
		logFactory = logging.NewFactory(logging.Config{
			DisplayLevel: logging.Debug,
		})
		l, err := logFactory.Make("startExternalSubscriberCommand")
		if err != nil {
			return err
		}
		log = l

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

		externalSubscriberServer := externalsubscriber.NewExternalSubscriberServer(log, ParserFactory, acceptedSubscribers)
		grpcHandler, err := externalsubscriber.NewGRPCHandler(externalSubscriberServer, log, externalSubscriberPort)
		if err != nil {
			return err
		}
		grpcHandler.Start()

		<-signals
		grpcHandler.Stop()
		log.Info("\nShutting down...")
		return nil
	},
}

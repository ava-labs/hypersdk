// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/extension/grpcindexer"
	"github.com/ava-labs/hypersdk/extension/indexer"
	"github.com/ava-labs/hypersdk/extension/splitter"

	lsplitter "github.com/ava-labs/hypersdk/examples/morpheusvm/splitter"
)

var (
	logFactory          logging.Factory
	log                 logging.Logger
	acceptedSubscribers *indexer.AcceptedSubscribers
	tcpPort             string // Must be of format ":<PORT-NUM>"
)

func init() {
	// Initialize logger
	logFactory = logging.NewFactory(logging.Config{
		DisplayLevel: logging.Debug,
	})
	l, err := logFactory.Make("main")
	if err != nil {
		panic(err)
	}
	log = l
	// Initialize acceptedSubscribers
	var acceptedSubscriberList []indexer.AcceptedSubscriber
	acceptedSubscribers = indexer.NewAcceptedSubscribers(acceptedSubscriberList...)
	// Initialize TCP port
	tcpPort = ":9001"
}

func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	externalSubscriberServer := grpcindexer.NewExternalSubscriberServer(log, lsplitter.ParserFactory, acceptedSubscribers)
	splitter := splitter.NewSplitter(externalSubscriberServer, log, tcpPort)
	splitter.Start()

	<-signals
	splitter.Stop()
	log.Info("\nShutting down...")
}

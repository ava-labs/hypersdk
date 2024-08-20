// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"encoding/json"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"google.golang.org/grpc"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/extension/grpcindexer"
	"github.com/ava-labs/hypersdk/extension/indexer"

	pb "github.com/ava-labs/hypersdk/proto/pb/externalsubscriber"
)

var (
	logFactory          logging.Factory
	log                 logging.Logger
	acceptedSubscribers *indexer.AcceptedSubscribers
)

// Used as a lambda function for creating ExternalSubscriberServer parser
func ParserFactory(networkID uint32, chainID ids.ID, genesisBytes []byte) (chain.Parser, error) {
	var genesis genesis.Genesis
	if err := json.Unmarshal(genesisBytes, &genesis); err != nil {
		return nil, err
	}
	parser := rpc.NewParser(networkID, chainID, &genesis)
	return parser, nil
}

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
}

func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	externalSubscriberServer := grpcindexer.NewExternalSubscriberServer(log, ParserFactory, acceptedSubscribers)

	go startGRPCServer(externalSubscriberServer)

	<-signals
	log.Info("\nShutting down...")
}

func startGRPCServer(m *grpcindexer.ExternalSubscriberServer) {
	lis, err := net.Listen("tcp", ":9001") // #nosec G102
	if err != nil {
		log.Fatal("Could not listen to TCP port 9001")
	}
	log.Info("listening to TCP port 9001")

	s := grpc.NewServer()
	pb.RegisterExternalSubscriberServer(s, m)
	log.Info("Server listening on port 9001")

	if err := s.Serve(lis); err != nil {
		log.Fatal("failed to serve TCP port 9001")
	}
}

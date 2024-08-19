// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ava-labs/avalanchego/ids"
	"google.golang.org/grpc"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/extension/grpcindexer"

	pb "github.com/ava-labs/hypersdk/proto"
)

// Used as a lambda function for creating ExternalSubscriberServer parser
func ParserFactory(networkID uint32, chainID ids.ID, genBytes []byte) (chain.Parser, error) {
	var genesis genesis.Genesis
	if err := json.Unmarshal(genBytes, &genesis); err != nil {
		return nil, err
	}
	parser := rpc.NewParser(networkID, chainID, &genesis)
	return parser, nil
}

func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	stdDBDir := "./stdDB/"
	logger := log.Default()

	externalSubscriberServer := grpcindexer.NewExternalSubscriberServer(stdDBDir, logger, ParserFactory)

	go startGRPCServer(externalSubscriberServer)

	<-signals
	externalSubscriberServer.Logger.Println("\nShutting down...")
}

func startGRPCServer(m *grpcindexer.ExternalSubscriberServer) {
	lis, err := net.Listen("tcp", ":9001") // #nosec G102
	if err != nil {
		m.Logger.Fatalln("could not listen to TCP port 9001")
	}
	m.Logger.Println("listening to TCP port 9001")

	s := grpc.NewServer()
	pb.RegisterExternalSubscriberServer(s, m)
	m.Logger.Println("Server listening on port 9001")

	if err := s.Serve(lis); err != nil {
		m.Logger.Fatalln("failed to serve TCP port 9001")
	}
}

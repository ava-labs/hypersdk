// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/gorilla/rpc/v2"
	"google.golang.org/grpc"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/externalsubscriber"

	pb "github.com/ava-labs/hypersdk/proto"
)

// Spins up a Sidecar server (gRPC server and API server)
func main() {
	signals := make(chan os.Signal, 1)
    signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	stdDBDir := "./stdDB/"
	logger := log.Default()

	morpheusSidecar := externalsubscriber.NewMorpheusSidecar(stdDBDir, logger)

	go startGRPCServer(morpheusSidecar)
	go startMorpheusAPI(morpheusSidecar)

	// Wait for a signal
    <-signals
	morpheusSidecar.Logger.Println("\nShutting down...")
}

func startGRPCServer(m *externalsubscriber.MorpheusSidecar) {
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

func startMorpheusAPI(m *externalsubscriber.MorpheusSidecar) {

	codec := json.NewCodec()

	rpcServer := rpc.NewServer()
	rpcServer.RegisterCodec(codec, "application/json")
	rpcServer.RegisterCodec(codec, "application/json;charset=UTF-8")

	if err := rpcServer.RegisterService(m, "mei"); err != nil {
		m.Logger.Fatalln("Could not register indexer-rpc service")
	}

	http.Handle("/rpc", rpcServer)

	m.Logger.Println("Starting API service at port 8080")
	m.Logger.Fatalln(http.ListenAndServe(":8080", rpcServer)) // #nosec G114
}

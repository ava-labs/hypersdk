// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"log"
	"net"
	"net/http"

	"google.golang.org/grpc"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/externalsubscriber"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/externalsubscriber/router"

	pb "github.com/ava-labs/hypersdk/proto"
)

// Spins up a Sidecar server (gRPC server and API server)
func main() {
	stdDBDir := "./stdDB/"
	logger := log.Default()

	morpheusSidecar := externalsubscriber.NewMorpheusSidecar(stdDBDir, logger)

	go startGRPCServer(morpheusSidecar)
	go startMorpheusAPI(morpheusSidecar)

	select {}
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
	r := router.NewRouter()

	if err := r.AddRouter("/indexer-rpc", "", m); err != nil {
		m.Logger.Fatalln("Could not create /indexer-rpc endpoint")
	}

	m.Logger.Println("Starting API service at port 8080")
	m.Logger.Fatalln(http.ListenAndServe(":8080", r)) // #nosec G114
}

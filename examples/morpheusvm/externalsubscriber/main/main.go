// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"log"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/externalsubscriber"

	mpb "github.com/ava-labs/hypersdk/examples/morpheusvm/proto"
	pb "github.com/ava-labs/hypersdk/proto"
)

var logger = log.Default()

// Spins up a Sidecar server (gRPC server and API server)
func main() {
	morpheusSidecar := externalsubscriber.NewMorpheusSidecar()

	go startGRPCServer(morpheusSidecar)
	go startMorpheusAPI(morpheusSidecar)

	select {}
}

func startGRPCServer(m *externalsubscriber.MorpheusSidecar) {
	lis, err := net.Listen("tcp", ":9001") // #nosec G102
	if err != nil {
		logger.Fatalln("could not listen to TCP port 9001")
	}
	logger.Println("listening to TCP port 9001")

	s := grpc.NewServer()
	mpb.RegisterMorpheusSubscriberServer(s, m)
	pb.RegisterExternalSubscriberServer(s, m)
	logger.Println("Server listening on port 9001")

	if err := s.Serve(lis); err != nil {
		logger.Fatalln("failed to serve TCP port 9001")
	}
}

func startMorpheusAPI(m *externalsubscriber.MorpheusSidecar) {
	router := mux.NewRouter()

	router.HandleFunc("/block", m.GetBlock).Methods("GET")
	router.HandleFunc("/tx", m.GetTX).Methods("GET")

	logger.Println("Starting API at port 8080")
	logger.Fatalln(http.ListenAndServe(":8080", router)) // #nosec G114
}

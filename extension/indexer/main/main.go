package main

import (
	"log"
	"net"

	"github.com/ava-labs/hypersdk/extension/indexer"
	pb "github.com/ava-labs/hypersdk/proto"
	"google.golang.org/grpc"
)

var logger = log.Default()

// Spins up a Sidecar server
func main() {
	lis, err := net.Listen("tcp", ":9001")
	if err != nil {
		logger.Fatalln("could not listen to TCP port 9001")
	}
	logger.Println("listening to TCP port 9001")

	s := grpc.NewServer()
	pb.RegisterExternalSubscriberServer(s, &indexer.Sidecar{})

	logger.Println("Server listening on port 9001")

	if err := s.Serve(lis); err != nil {
		logger.Fatalln("failed to serve TCP port 9001")
	}
}
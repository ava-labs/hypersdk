package main

import (
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/externalsubscriber"

	mpb "github.com/ava-labs/hypersdk/examples/morpheusvm/proto"
	pb "github.com/ava-labs/hypersdk/proto"
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
	// pb.RegisterExternalSubscriberServer(s,
	// &external_subscriber.MorpheusSidecar{})
	morpheusSidecar := externalsubscriber.NewMorpheusSidecar()
	mpb.RegisterMorpheusSubscriberServer(s, morpheusSidecar)
	pb.RegisterExternalSubscriberServer(s, morpheusSidecar)

	logger.Println("Server listening on port 9001")

	if err := s.Serve(lis); err != nil {
		logger.Fatalln("failed to serve TCP port 9001")
	}
}

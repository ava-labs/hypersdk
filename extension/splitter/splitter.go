// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package splitter

import (
	"net"

	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/ava-labs/hypersdk/extension/grpcindexer"

	pb "github.com/ava-labs/hypersdk/proto/pb/externalsubscriber"
)

type Splitter struct {
	gRPCServer *grpc.Server
	listener   net.Listener
	tcpPort    string
	log        logging.Logger
}

func NewSplitter(e *grpcindexer.ExternalSubscriberServer, log logging.Logger, tcpPort string) *Splitter {
	lis, err := net.Listen("tcp", tcpPort) // #nosec G102
	if err != nil {
		log.Fatal("Could not listen to TCP port", zap.Any("Address", tcpPort))
	}
	log.Info("listening to TCP port", zap.Any("Address", tcpPort))

	s := grpc.NewServer()
	pb.RegisterExternalSubscriberServer(s, e)
	return &Splitter{
		gRPCServer: s,
		listener:   lis,
		tcpPort:    tcpPort,
		log:        log,
	}
}

func (s *Splitter) Start() {
	go func() {
		if err := s.gRPCServer.Serve(s.listener); err != nil {
			s.log.Fatal("failed to serve TCP port", zap.Any("Address", s.tcpPort))
		}
	}()
}

func (s *Splitter) Stop() {
	s.gRPCServer.GracefulStop()
	if err := s.listener.Close(); err != nil {
		s.log.Info("failed to close listener", zap.Any("error", err))
	}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"net"

	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	pb "github.com/ava-labs/hypersdk/proto/pb/externalsubscriber"
)

type GRPCHandler struct {
	gRPCServer *grpc.Server
	listener   net.Listener
	tcpPort    string
	log        logging.Logger
}

func NewGRPCHandler(e *ExternalSubscriberServer, log logging.Logger, tcpPort string) (*GRPCHandler, error) {
	lis, err := net.Listen("tcp", tcpPort) // #nosec G102
	if err != nil {
		return nil, err
	}
	log.Info("listening to tcp port", zap.String("address", tcpPort))

	s := grpc.NewServer()
	pb.RegisterExternalSubscriberServer(s, e)
	return &GRPCHandler{
		gRPCServer: s,
		listener:   lis,
		tcpPort:    tcpPort,
		log:        log,
	}, nil
}

func (g *GRPCHandler) Start() {
	go func() {
		if err := g.gRPCServer.Serve(g.listener); err != nil {
			g.log.Fatal("failed to serve tcp port",
				zap.String("address", g.tcpPort),
				zap.Any("err", err),
			)
		}
	}()
}

func (g *GRPCHandler) Stop() {
	g.gRPCServer.GracefulStop()
	if err := g.listener.Close(); err != nil {
		g.log.Debug("failed to close listener", zap.Any("error", err))
	}
}

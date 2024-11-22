// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"

	pb "github.com/ava-labs/hypersdk/proto/pb/externalsubscriber"
)

type CreateParser func(genesisBytes []byte) (chain.Parser, error)

var (
	ErrParserNotInitialized     = errors.New("parser not initialized")
	ErrParserAlreadyInitialized = errors.New("parser already initialized")
)

type ExternalSubscriberServer struct {
	pb.ExternalSubscriberServer
	parser              chain.Parser
	createParser        CreateParser
	acceptedSubscribers []event.Subscription[*chain.ExecutedBlock]
	log                 logging.Logger
}

func NewExternalSubscriberServer(
	logger logging.Logger,
	createParser CreateParser,
	acceptedSubscribers []event.Subscription[*chain.ExecutedBlock],
) *ExternalSubscriberServer {
	return &ExternalSubscriberServer{
		log:                 logger,
		createParser:        createParser,
		acceptedSubscribers: acceptedSubscribers,
	}
}

func (e *ExternalSubscriberServer) Initialize(_ context.Context, initRequest *pb.InitializeRequest) (*emptypb.Empty, error) {
	if e.parser != nil {
		return &emptypb.Empty{}, ErrParserAlreadyInitialized
	}
	// Create parser and store
	parser, err := e.createParser(initRequest.Genesis)
	if err != nil {
		return &emptypb.Empty{}, err
	}
	e.parser = parser
	e.log.Info("initialized external subscriber parser")
	return &emptypb.Empty{}, nil
}

func (e *ExternalSubscriberServer) Notify(ctx context.Context, b *pb.BlockRequest) (*emptypb.Empty, error) {
	// Continue only if we can parse
	if e.parser == nil {
		return &emptypb.Empty{}, ErrParserNotInitialized
	}
	blk, err := chain.UnmarshalExecutedBlock(b.BlockData, e.parser)
	if err != nil {
		return &emptypb.Empty{}, err
	}

	e.log.Info("external subscriber received accepted block",
		zap.Uint64("height", blk.Block.Hght),
	)

	// Forward to subscribers
	for _, s := range e.acceptedSubscribers {
		if err := s.Notify(ctx, blk); err != nil {
			return &emptypb.Empty{}, err
		}
	}
	return &emptypb.Empty{}, nil
}

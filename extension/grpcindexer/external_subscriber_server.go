// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package grpcindexer

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/extension/indexer"

	pb "github.com/ava-labs/hypersdk/proto/pb/externalsubscriber"
)

type ParserFactory func(networkID uint32, chainID ids.ID, genesisBytes []byte) (chain.Parser, error)

var (
	ErrParserNotInitialized     = errors.New("parser not initialized")
	ErrParserAlreadyInitialized = errors.New("parser already initialized")
)

type ExternalSubscriberServer struct {
	pb.ExternalSubscriberServer
	parser              chain.Parser
	parserFactory       ParserFactory
	acceptedSubscribers *indexer.AcceptedSubscribers
	log                 logging.Logger
}

func NewExternalSubscriberServer(
	logger logging.Logger,
	parserFactory ParserFactory,
	acceptedSubscribers *indexer.AcceptedSubscribers,
) *ExternalSubscriberServer {
	return &ExternalSubscriberServer{
		log:                 logger,
		parserFactory:       parserFactory,
		acceptedSubscribers: acceptedSubscribers,
	}
}

func (e *ExternalSubscriberServer) Initialize(_ context.Context, initRequest *pb.InitRequest) (*emptypb.Empty, error) {
	// TODO: return error instead of nil if trying to initialize more than once
	if e.parser != nil {
		return &emptypb.Empty{}, ErrParserAlreadyInitialized
	}
	// Unmarshal chainID
	chainID := ids.ID(initRequest.ChainId)
	// Create parser and store
	parser, err := e.parserFactory(initRequest.NetworkId, chainID, initRequest.Genesis)
	if err != nil {
		return &emptypb.Empty{}, err
	}
	e.parser = parser
	e.log.Info("External Subscriber has initialized the parser associated with MorpheusVM")
	return &emptypb.Empty{}, nil
}

func (e *ExternalSubscriberServer) ProcessBlock(ctx context.Context, b *pb.BlockRequest) (*emptypb.Empty, error) {
	// Continue only if we can parse
	if e.parser == nil {
		return &emptypb.Empty{}, ErrParserNotInitialized
	}
	blk, err := chain.UnmarshalBlock(b.BlockData, e.parser)
	if err != nil {
		return &emptypb.Empty{}, err
	}

	results, err := chain.UnmarshalResults(b.Results)
	if err != nil {
		return &emptypb.Empty{}, err
	}

	e.log.Info("Indexing block from VM", zap.Any("Block Height", blk.Hght))

	return &emptypb.Empty{}, e.acceptedSubscribers.Accepted(ctx, blk, results)
}

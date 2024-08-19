// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package grpcindexer

import (
	"context"
	"log"

	"github.com/ava-labs/avalanchego/ids"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/extension/indexer"
	"github.com/ava-labs/hypersdk/pebble"

	avametrics "github.com/ava-labs/avalanchego/api/metrics"
	pb "github.com/ava-labs/hypersdk/proto"
	hstorage "github.com/ava-labs/hypersdk/storage"
)

type ParserFactory func(networkID uint32, chainID ids.ID, genBytes []byte) (chain.Parser, error)

type ExternalSubscriberServer struct {
	pb.ExternalSubscriberServer
	indexer       indexer.Indexer
	parser        chain.Parser
	parserFactory ParserFactory
	Logger        *log.Logger
}

func NewExternalSubscriberServer(stdDBDir string, logger *log.Logger, parserFactory ParserFactory) *ExternalSubscriberServer {
	stdDB, err := hstorage.New(pebble.NewDefaultConfig(), stdDBDir, "db", avametrics.NewLabelGatherer("standardIndexer"))
	if err != nil {
		logger.Fatalln("Failed to create DB for standard indexer")
	}
	return &ExternalSubscriberServer{
		indexer:       indexer.NewDBIndexer(stdDB),
		Logger:        logger,
		parserFactory: parserFactory,
	}
}

func (e *ExternalSubscriberServer) Initialize(_ context.Context, initRequest *pb.InitRequest) (*emptypb.Empty, error) {
	if e.parser != nil {
		return &emptypb.Empty{}, nil
	}

	// Unmarshal chainID
	chainID := ids.ID(initRequest.ChainID)
	// Create parser and store
	parser, err := e.parserFactory(initRequest.NetworkID, chainID, initRequest.Genesis)
	if err != nil {
		e.Logger.Println("Unable to create parser", err)
		return &emptypb.Empty{}, err
	}
	e.parser = parser
	e.Logger.Println("External Subscriber has initialized the parser associated with MorpheusVM")
	return &emptypb.Empty{}, nil
}

func (e *ExternalSubscriberServer) ProcessBlock(ctx context.Context, b *pb.BlockRequest) (*emptypb.Empty, error) {
	if e.parser == nil {
		return &emptypb.Empty{}, nil
	}

	// Unmarshal block
	blk, err := chain.UnmarshalBlock(b.BlockData, e.parser)
	if err != nil {
		return &emptypb.Empty{}, err
	}

	// Unmarshal results
	results, err := chain.UnmarshalResults(b.Results)
	if err != nil {
		return &emptypb.Empty{}, err
	}

	// Index block
	if err := e.indexer.AcceptedStateful(ctx, blk, results); err != nil {
		return &emptypb.Empty{}, err
	}
	e.Logger.Println("Indexed block number ", blk.Hght)
	return &emptypb.Empty{}, nil
}

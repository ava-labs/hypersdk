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

type CreateParser[T chain.RuntimeInterface] func(genesisBytes []byte) (chain.Parser[T], error)

var (
	ErrParserNotInitialized     = errors.New("parser not initialized")
	ErrParserAlreadyInitialized = errors.New("parser already initialized")
)

// TODO: switch to eventually using chain.Stateless block
// Wrapper to pass blocks + results to subscribers
type ExternalSubscriberSubscriptionData[T chain.RuntimeInterface] struct {
	Blk     *chain.StatelessBlock[T]
	Results []*chain.Result
}

func NewExternalSubscriberSubscriptionData[T chain.RuntimeInterface](
	blk *chain.StatelessBlock[T],
	results []*chain.Result,
) *ExternalSubscriberSubscriptionData[T] {
	return &ExternalSubscriberSubscriptionData[T]{
		Blk:     blk,
		Results: results,
	}
}

type ExternalSubscriberServer[T chain.RuntimeInterface] struct {
	pb.ExternalSubscriberServer
	parser              chain.Parser[T]
	createParser        CreateParser[T]
	acceptedSubscribers []event.Subscription[*ExternalSubscriberSubscriptionData[T]]
	log                 logging.Logger
}

func NewExternalSubscriberServer[T chain.RuntimeInterface](
	logger logging.Logger,
	createParser CreateParser[T],
	acceptedSubscribers []event.Subscription[*ExternalSubscriberSubscriptionData[T]],
) *ExternalSubscriberServer[T] {
	return &ExternalSubscriberServer[T]{
		log:                 logger,
		createParser:        createParser,
		acceptedSubscribers: acceptedSubscribers,
	}
}

func (e *ExternalSubscriberServer[_]) Initialize(_ context.Context, initRequest *pb.InitializeRequest) (*emptypb.Empty, error) {
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

func (e *ExternalSubscriberServer[_]) AcceptBlock(_ context.Context, b *pb.BlockRequest) (*emptypb.Empty, error) {
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

	e.log.Info("external subscriber received accepted block",
		zap.Uint64("height", blk.Hght),
	)

	// Forward block + results
	externalSubscriberSubscriptionData := NewExternalSubscriberSubscriptionData(blk, results)
	for _, s := range e.acceptedSubscribers {
		if err := s.Accept(externalSubscriberSubscriptionData); err != nil {
			return &emptypb.Empty{}, err
		}
	}
	return &emptypb.Empty{}, nil
}

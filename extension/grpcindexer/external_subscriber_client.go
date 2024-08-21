// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package grpcindexer

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/extension/indexer"

	pb "github.com/ava-labs/hypersdk/proto/pb/externalsubscriber"
)

var _ indexer.AcceptedSubscriber = (*ExternalSubscriberClient)(nil)

type ExternalSubscriberClient struct {
	client pb.ExternalSubscriberClient
	log    logging.Logger
}

func NewExternalSubscriberClient(
	ctx context.Context,
	log logging.Logger,
	serverAddr string,
	networkID uint32,
	chainID ids.ID,
	genesisBytes []byte,
) (*ExternalSubscriberClient, error) {
	// Establish connection to server
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := pb.NewExternalSubscriberClient(conn)
	// Initialize parser
	_, err = client.Initialize(
		ctx,
		&pb.InitRequest{
			NetworkId: networkID,
			ChainId:   chainID[:],
			Genesis:   genesisBytes,
		},
	)
	if err != nil {
		return nil, err
	}
	log.Debug("Established connection to External Subscriber Server", zap.Any("Server Address", serverAddr))
	return &ExternalSubscriberClient{
		client: client,
		log:    log,
	}, nil
}

func (e *ExternalSubscriberClient) Accepted(ctx context.Context, blk *chain.StatefulBlock, results []*chain.Result) error {
	// Make gRPC call to client
	e.log.Debug("Sending block over to server", zap.Any("Block", blk))
	blockBytes, err := blk.Marshal()
	if err != nil {
		return err
	}

	resultsMarshaled, err := chain.MarshalResults(results)
	if err != nil {
		return err
	}

	req := &pb.BlockRequest{
		BlockData: blockBytes,
		Results:   resultsMarshaled,
	}
	_, err = e.client.ProcessBlock(ctx, req)
	return err
}

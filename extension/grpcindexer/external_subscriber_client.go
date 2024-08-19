// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package grpcindexer

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/extension/indexer"

	pb "github.com/ava-labs/hypersdk/proto"
)

var _ indexer.AcceptedSubscriber = (*ExternalSubscriberClient)(nil)

type ExternalSubscriberClient struct {
	client pb.ExternalSubscriberClient
}

func NewExternalSubscriberClient(ctx context.Context, server string, networkID uint32, chainID ids.ID, genBytes []byte) (*ExternalSubscriberClient, error) {
	// Establish connection to server
	conn, err := grpc.Dial(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := pb.NewExternalSubscriberClient(conn)
	// Initialize parser
	_, err = client.Initialize(
		ctx,
		&pb.InitRequest{
			NetworkID: networkID,
			ChainID:   chainID[:],
			Genesis:   genBytes,
		},
	)
	if err != nil {
		return nil, err
	}
	return &ExternalSubscriberClient{
		client: client,
	}, nil
}

func (e *ExternalSubscriberClient) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	// Make gRPC call to client
	blockBytes, err := blk.Marshal()
	if err != nil {
		return err
	}

	resultsMarshaled, err := chain.MarshalResults(blk.Results())
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

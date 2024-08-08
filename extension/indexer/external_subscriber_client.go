// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ava-labs/hypersdk/chain"

	pb "github.com/ava-labs/hypersdk/proto"
)

var _ AcceptedSubscriber = (*ExternalSubscriber)(nil)

type ExternalSubscriber struct {
	client pb.ExternalSubscriberClient
}

func NewExternalSubscriber(server string) (*ExternalSubscriber, error) {
	conn, err := grpc.Dial(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &ExternalSubscriber{
		client: pb.NewExternalSubscriberClient(conn),
	}, nil
}

func (e *ExternalSubscriber) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	// Make gRPC call to client
	blockBytes, err := blk.Marshal()
	if err != nil {
		return err
	}
	req := &pb.BlockRequest{
		BlockData: blockBytes,
	}
	_, err = e.client.ProcessBlock(ctx, req)
	return err
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"log"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/ava-labs/hypersdk/chain"

	pb "github.com/ava-labs/hypersdk/proto"
)

var _ AcceptedSubscriber = (*ExternalSubscriber)(nil)

type ExternalSubscriber struct {
	client pb.ExternalSubscriberClient
}

func NewExternalSubscriber(server string) (*ExternalSubscriber, error) {
	conn, err := grpc.Dial(server, grpc.WithBlock(), grpc.WithInsecure())
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
		log.Fatal("Failed to marshal block", zap.Any("block", blk))
	}
	req := &pb.Block{
		BlockData: blockBytes,
	}
	_, err = e.client.ProcessBlock(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

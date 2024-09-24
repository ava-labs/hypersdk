// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"

	pb "github.com/ava-labs/hypersdk/proto/pb/externalsubscriber"
)

var _ event.Subscription[*chain.StatefulBlock[chain.PendingView]] = (*ExternalSubscriberClient[chain.PendingView])(nil)

type ExternalSubscriberClient[T chain.PendingView] struct {
	conn   *grpc.ClientConn
	client pb.ExternalSubscriberClient
	log    logging.Logger
}

func NewExternalSubscriberClient[T chain.PendingView](
	ctx context.Context,
	log logging.Logger,
	serverAddr string,
	genesisBytes []byte,
) (*ExternalSubscriberClient[T], error) {
	// Establish connection to server
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := pb.NewExternalSubscriberClient(conn)
	// Initialize parser
	_, err = client.Initialize(
		ctx,
		&pb.InitializeRequest{
			Genesis: genesisBytes,
		},
	)
	if err != nil {
		return nil, err
	}
	log.Debug("connected to external subscriber server", zap.String("address", serverAddr))
	return &ExternalSubscriberClient[T]{
		conn:   conn,
		client: client,
		log:    log,
	}, nil
}

func (e *ExternalSubscriberClient[T]) Accept(blk *chain.StatefulBlock[T]) error {
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
	e.log.Debug("sending accepted block to server",
		zap.Stringer("blockID", blk.ID()),
		zap.Uint64("blockHeight", blk.Hght),
	)
	_, err = e.client.AcceptBlock(context.TODO(), req)
	return err
}

func (e *ExternalSubscriberClient[_]) Close() error {
	return e.conn.Close()
}

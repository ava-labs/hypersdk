package indexer

import (
	"context"
	"encoding/json"

	"github.com/ava-labs/hypersdk/chain"
	pb "github.com/ava-labs/hypersdk/proto"
	"google.golang.org/grpc"
)

var _ AcceptedSubscriber = (*ExternalSubscriber)(nil)

type ExternalSubscriber struct{
	client pb.ExternalSubscriberClient
}

func NewExternalSubscriber(server string) (*ExternalSubscriber, error) {
	conn, err := grpc.Dial(server, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	logger.Println("Connected to: ", server)
	return &ExternalSubscriber{
		client: pb.NewExternalSubscriberClient(conn),
	}, nil
}

func (e *ExternalSubscriber) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	// Make gRPC call to client
	blockJSON, _ := json.Marshal(blk)
	req := &pb.Block{BlockData: string(blockJSON)}
	_, err := e.client.ProcessBlock(ctx, req)
	if err != nil {
		return err
	}

	return nil	
}

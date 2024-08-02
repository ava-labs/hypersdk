package indexer

import (
	"context"
	"log"

	pb "github.com/ava-labs/hypersdk/proto"
)

var logger = log.Default()

type Sidecar struct {
	pb.UnimplementedExternalSubscriberServer
}

func (s *Sidecar) ProcessBlock(_ context.Context, blk *pb.Block) (*pb.BlockAck, error) {	

	logger.Println("External Indexer got the following block: ", blk.BlockData)

	return &pb.BlockAck{Success: true}, nil
}
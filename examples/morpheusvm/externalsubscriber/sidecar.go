// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"
	"encoding/json"
	"log"

	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/extension/indexer"
	"github.com/ava-labs/hypersdk/pebble"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	mpb "github.com/ava-labs/hypersdk/examples/morpheusvm/proto"
	pb "github.com/ava-labs/hypersdk/proto"
	hstorage "github.com/ava-labs/hypersdk/storage"
)

// TODO: clean up directory structure
var stdDBDir = "./stdDB/"

type MorpheusSidecar struct {
	pb.ExternalSubscriberServer
	mpb.MorpheusSubscriberServer
	standardIndexer indexer.StandardIndexer
	parser          *rpc.Parser
}

func NewMorpheusSidecar() *MorpheusSidecar {
	stdDB, err := hstorage.New(pebble.NewDefaultConfig(), stdDBDir, "db", ametrics.NewLabelGatherer("standardIndexer"))
	if err != nil {
		log.Fatalln("Failed to create DB for standard indexer")
	}
	return &MorpheusSidecar{standardIndexer: indexer.NewStandardDBIndexer(stdDB)}
}

func (m *MorpheusSidecar) Initialize(_ context.Context, initMsg *mpb.InitMsg) (*mpb.InitMsgAck, error) {
	if m.parser != nil {
		// TODO: implement better error handling for case where parser is
		// already initialized
		return &mpb.InitMsgAck{Success: true}, nil
	}

	// Unmarshal chainID, genesis
	chainID := ids.ID(initMsg.ChainID)
	var gen genesis.Genesis
	if err := json.Unmarshal(initMsg.Genesis, &gen); err != nil {
		logger.Println("Unable to unmarhsal genesis", zap.Any("genesis", initMsg.Genesis))
		return nil, err
	}
	m.parser = rpc.NewParser(initMsg.NetworkID, chainID, &gen)
	logger.Println("External Subscriber has initialized the parser associated with MorpheusVM")
	return &mpb.InitMsgAck{Success: true}, nil
}

func (m *MorpheusSidecar) ProcessBlock(ctx context.Context, b *pb.Block) (*pb.BlockAck, error) {
	if m.parser == nil {
		return &pb.BlockAck{Success: true}, nil
	}

	// Unmarshal block
	blk, err := chain.UnmarshalBlock(b.BlockData, m.parser)
	if err != nil {
		log.Fatalln("Failed to unmarshal block", zap.Any("block", b.BlockData))
	}

	// Call accept only if block has been indexed
	if m.standardIndexer.BlockAlreadyIndexed(blk.Hght) {
		return &pb.BlockAck{Success: true}, nil
	}

	// Index block
	if err := m.standardIndexer.AcceptedStateful(ctx, blk); err != nil {
		return &pb.BlockAck{Success: false}, nil
	}
	logger.Println("Indexed block number ", blk.Hght)
	return &pb.BlockAck{Success: true}, nil
}

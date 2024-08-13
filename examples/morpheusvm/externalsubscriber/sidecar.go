// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/extension/indexer"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/pebble"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	pb "github.com/ava-labs/hypersdk/proto"
	hstorage "github.com/ava-labs/hypersdk/storage"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type GetBlockRequest struct {
	Id ids.ID `json:"id"`
}

type GetBlockByHeightRequest struct {
	Height uint64 `json:"height"`
}

type GetBlockResponse struct {
	Block *chain.StatefulBlock `json:"block"`
}

type GetTxRequest struct {
	TxID ids.ID `json:"txID"`
}

type GetTXResponse struct {
	Exists        bool            `json:"exists"`
	Timestamp     int64           `json:"timestamp"`
	Success       bool            `json:"success"`
	FeeDimensions fees.Dimensions `json:"feeDimensions"`
	Fee           uint64          `json:"fee"`
}

type MorpheusSidecar struct {
	pb.ExternalSubscriberServer
	standardIndexer indexer.Indexer
	parser          *rpc.Parser
	Logger          *log.Logger
}

func NewMorpheusSidecar(stdDBDir string, logger *log.Logger) *MorpheusSidecar {
	stdDB, err := hstorage.New(pebble.NewDefaultConfig(), stdDBDir, "db", ametrics.NewLabelGatherer("standardIndexer"))
	if err != nil {
		log.Fatalln("Failed to create DB for standard indexer")
	}
	return &MorpheusSidecar{
		standardIndexer: indexer.NewDBIndexer(stdDB),
		Logger:          logger,
	}
}

/*
gRPC-related functions
*/

func (m *MorpheusSidecar) Initialize(_ context.Context, initRequest *pb.InitRequest) (*emptypb.Empty, error) {
	if m.parser != nil {
		return &emptypb.Empty{}, nil
	}

	// Unmarshal chainID, genesis
	chainID := ids.ID(initRequest.ChainID)
	var gen genesis.Genesis
	if err := json.Unmarshal(initRequest.Genesis, &gen); err != nil {
		m.Logger.Println("Unable to unmarhsal genesis", zap.Any("genesis", initRequest.Genesis))
		return nil, err
	}
	m.parser = rpc.NewParser(initRequest.NetworkID, chainID, &gen)
	m.Logger.Println("External Subscriber has initialized the parser associated with MorpheusVM")
	return &emptypb.Empty{}, nil
}

func (m *MorpheusSidecar) ProcessBlock(ctx context.Context, b *pb.BlockRequest) (*emptypb.Empty, error) {
	if m.parser == nil {
		return &emptypb.Empty{}, nil
	}

	// Unmarshal block
	blk, err := chain.UnmarshalBlock(b.BlockData, m.parser)
	if err != nil {
		return &emptypb.Empty{}, err
	}

	// Index block
	if err := m.standardIndexer.AcceptedStateful(ctx, blk); err != nil {
		return &emptypb.Empty{}, nil
	}
	m.Logger.Println("Indexed block number ", blk.Hght)
	return &emptypb.Empty{}, nil
}

/*
JSON-RPC related functions
*/

func (m *MorpheusSidecar) GetBlock(_ *http.Request, args *GetBlockRequest, resp *GetBlockResponse) error {
	blk, err := m.standardIndexer.GetBlock(args.Id)
	if err != nil {
		m.Logger.Println("Could not get block", zap.Any("ID", args.Id))
		return err
	}

	uBlk, err := chain.UnmarshalBlock(blk, m.parser)
	if err != nil {
		m.Logger.Println("Could not unmarshal block", zap.Any("Block Bytes", blk))
	}
	
	resp.Block = uBlk

	return nil
}

func (m *MorpheusSidecar) GetBlockByHeight(_ *http.Request, args *GetBlockByHeightRequest, resp *GetBlockResponse) error {
	blk, err := m.standardIndexer.GetBlockByHeight(args.Height)
	if err != nil {
		m.Logger.Println("Could not get block", zap.Any("ID", args.Height))
		return err
	}
	
	uBlk, err := chain.UnmarshalBlock(blk, m.parser)
	if err != nil {
		m.Logger.Println("Could not unmarshal block", zap.Any("Block Bytes", blk))
	}
	
	resp.Block = uBlk

	return nil
}

func (m *MorpheusSidecar) GetTX(_ *http.Request, args *GetTxRequest, resp *GetTXResponse) error {
	exists, timestamp, success, dim, fee, err := m.standardIndexer.GetTransaction(args.TxID)
	if err != nil {
		m.Logger.Println("Could not get TX", zap.Any("txID", args.TxID))
		return err
	}
	resp.Exists = exists
	resp.Timestamp = timestamp
	resp.Success = success
	resp.FeeDimensions = dim
	resp.Fee = fee

	return nil
}

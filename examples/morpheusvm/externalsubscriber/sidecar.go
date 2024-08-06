// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/extension/indexer"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/pebble"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	mpb "github.com/ava-labs/hypersdk/examples/morpheusvm/proto"
	pb "github.com/ava-labs/hypersdk/proto"
	hstorage "github.com/ava-labs/hypersdk/storage"
)

// TODO: clean up directory structure
var stdDBDir = "./stdDB/"

type ExternalMorpheusAPI interface {
	GetBlock(w http.ResponseWriter, r *http.Request)
	GetTX(w http.ResponseWriter, r *http.Request)
}

type MorpheusSidecar struct {
	pb.ExternalSubscriberServer
	mpb.MorpheusSubscriberServer
	ExternalMorpheusAPI
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

func (m *MorpheusSidecar) GetBlock(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	// Get block height
	blkHeight := queryParams.Get("height")
	num, err := strconv.ParseUint(blkHeight, 10, 64)
	if err != nil {
		logger.Println("Could not parse block height", zap.Any("Height", blkHeight))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Get block
	blk, err := m.standardIndexer.GetBlockByHeight(num)
	if err != nil {
		logger.Println("Could not get block", zap.Any("Height", num))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Unmarshal block
	uBlk, err := chain.UnmarshalBlock(blk, m.parser)
	if err != nil {
		logger.Println("Could not unmarshall block", zap.Any("Block Bytes", blk))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(uBlk); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type GetTXResponse struct {
	Exists        bool            `json:"exists"`
	Timestamp     int64           `json:"timestamp"`
	Success       bool            `json:"success"`
	FeeDimensions fees.Dimensions `json:"feeDimensions"`
	Fee           uint64          `json:"fee"`
}

func (m *MorpheusSidecar) GetTX(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	// Get TXID
	txID := queryParams.Get("txID")
	txIDConv, err := ids.FromString(txID)
	if err != nil {
		logger.Println("Could not convert txID", zap.Any("txID", txID))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	exists, timestamp, success, dim, fee, err := m.standardIndexer.GetTransaction(txIDConv)
	if err != nil {
		logger.Println("Could not get TX", zap.Any("txID", txIDConv))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := GetTXResponse{
		Exists:        exists,
		Timestamp:     timestamp,
		Success:       success,
		FeeDimensions: dim,
		Fee:           fee,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Println("Failed to encode response", zap.Any("Response", resp))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

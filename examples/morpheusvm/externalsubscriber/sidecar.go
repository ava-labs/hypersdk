// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/externalsubscriber/router"
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

var errUnknownMethod = errors.New("unknown rpc method")

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      interface{} `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type JSONRPCErrorResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Error   interface{} `json:"error"`
	ID      interface{} `json:"id"`
}

type GetBlockRequest struct {
	Height uint64 `json:"height"`
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
	http.Handler
	standardIndexer indexer.StandardIndexer
	parser          *rpc.Parser
	router          *router.Router
	Logger          *log.Logger
}

func NewMorpheusSidecar(stdDBDir string, logger *log.Logger) *MorpheusSidecar {
	stdDB, err := hstorage.New(pebble.NewDefaultConfig(), stdDBDir, "db", ametrics.NewLabelGatherer("standardIndexer"))
	if err != nil {
		log.Fatalln("Failed to create DB for standard indexer")
	}
	return &MorpheusSidecar{
		standardIndexer: indexer.NewStandardDBIndexer(stdDB),
		router:          router.NewRouter(),
		Logger:          logger,
	}
}

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

	// Call accept only if block has been indexed
	if m.standardIndexer.BlockAlreadyIndexed(blk.Hght) {
		return &emptypb.Empty{}, nil
	}

	// Index block
	if err := m.standardIndexer.AcceptedStateful(ctx, blk); err != nil {
		return &emptypb.Empty{}, nil
	}
	m.Logger.Println("Indexed block number ", blk.Hght)
	return &emptypb.Empty{}, nil
}

func (m *MorpheusSidecar) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid params", http.StatusBadRequest)
		return
	}
	switch req.Method {
	case "getBlock":
		var getBlockRequest GetBlockRequest
		if err := m.parseParams(req.Params, &getBlockRequest); err != nil {
			http.Error(w, "Invalid block params", http.StatusBadRequest)
			return
		}
		// Get block
		blk, err := m.standardIndexer.GetBlockByHeight(getBlockRequest.Height)
		if err != nil {
			m.Logger.Println("Could not get block", zap.Any("Height", getBlockRequest.Height))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Unmarshal block
		uBlk, err := chain.UnmarshalBlock(blk, m.parser)
		if err != nil {
			m.Logger.Println("Could not unmarshall block", zap.Any("Block Bytes", blk))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := m.sendResponse(w, req.ID, uBlk, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case "getTX":
		var getTxRequest GetTxRequest
		if err := m.parseParams(req.Params, &getTxRequest); err != nil {
			http.Error(w, "Invalid TX params", http.StatusBadRequest)
			return
		}
		exists, timestamp, success, dim, fee, err := m.standardIndexer.GetTransaction(getTxRequest.TxID)
		if err != nil {
			m.Logger.Println("Could not get TX", zap.Any("txID", getTxRequest.TxID))
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
		if err := m.sendResponse(w, req.ID, resp, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		http.Error(w, errUnknownMethod.Error(), http.StatusBadRequest)
	}
}

func (*MorpheusSidecar) parseParams(params interface{}, dest interface{}) error {
	// Convert params to JSON, then decode into the destination struct
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (*MorpheusSidecar) sendResponse(w http.ResponseWriter, id interface{}, result interface{}, errMsg interface{}) error {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		Error:   errMsg,
		ID:      id,
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(response)
}

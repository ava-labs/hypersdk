// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	brpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/eth"
	"github.com/ava-labs/subnet-evm/params"
	eth_rpc "github.com/ava-labs/subnet-evm/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

// evmRelayCmd flags
var (
	evmRelayPort    int
	evmRelayAddress string
)

type API struct {
	chainConfig *params.ChainConfig
	priv        *cli.PrivateKey
	factory     chain.AuthFactory
	cli         *rpc.JSONRPCClient
	bcli        *brpc.JSONRPCClient
	ws          *rpc.WebSocketClient

	lock    sync.Mutex
	created map[ids.ID]uint64
}

func (s *API) registerContractCreation(id ids.ID, nonce uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.created == nil {
		s.created = make(map[ids.ID]uint64)
	}
	s.created[id] = nonce
}

func (s *API) getContractCreation(id ids.ID) *common.Address {
	s.lock.Lock()
	defer s.lock.Unlock()

	nonce, found := s.created[id]
	if !found {
		return nil
	}
	evmAddress := actions.ToEVMAddress(s.priv.Address)
	contractAddress := crypto.CreateAddress(evmAddress, nonce)
	return &contractAddress
}

func (s *API) ChainId() *hexutil.Big {
	return (*hexutil.Big)(s.chainConfig.ChainID)
}

func (s *API) BlockNumber(ctx context.Context) (hexutil.Uint64, error) {
	_, number, _, err := s.cli.Accepted(ctx)
	utils.Outf("{{yellow}}BlockNumber: %d{{/}}\n", number)
	return hexutil.Uint64(number), err
}

func (s *API) GetBalance(ctx context.Context, address common.Address, blockNrOrHash eth_rpc.BlockNumberOrHash) (*hexutil.Big, error) {
	relayFor := common.HexToAddress(evmRelayAddress)
	if address == relayFor {
		address = actions.ToEVMAddress(s.priv.Address)
	}

	bal, _, err := s.bcli.EvmAccount(ctx, address.Hex())
	big := new(big.Int).SetUint64(bal)
	return (*hexutil.Big)(big), err
}

func (s *API) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash eth_rpc.BlockNumberOrHash) (*hexutil.Uint64, error) {
	relayFor := common.HexToAddress(evmRelayAddress)
	if address == relayFor {
		address = actions.ToEVMAddress(s.priv.Address)
	}

	_, nonce, err := s.bcli.EvmAccount(ctx, address.Hex())
	utils.Outf("{{yellow}}GetTransactionCount: %s %d{{/}}\n", address.Hex(), nonce)
	return (*hexutil.Uint64)(&nonce), err
}

func (s *API) GetBlockByNumber(ctx context.Context, number eth_rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	header := &types.Header{
		BaseFee: new(big.Int),
		Number:  big.NewInt(1),
	}
	block := types.NewBlock(header, nil, nil, nil, nil)
	return eth.RPCMarshalBlock(block, false, fullTx, s.chainConfig), nil
}

func (s *API) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	header := &types.Header{
		BaseFee: new(big.Int),
		Number:  big.NewInt(1),
	}
	block := types.NewBlock(header, nil, nil, nil, nil)
	return eth.RPCMarshalBlock(block, false, fullTx, s.chainConfig), nil
}

type feeHistoryResult struct {
	OldestBlock  *hexutil.Big     `json:"oldestBlock"`
	Reward       [][]*hexutil.Big `json:"reward,omitempty"`
	BaseFee      []*hexutil.Big   `json:"baseFeePerGas,omitempty"`
	GasUsedRatio []float64        `json:"gasUsedRatio"`
}

func (s *API) FeeHistory(ctx context.Context, blockCount math.HexOrDecimal64, lastBlock eth_rpc.BlockNumber, rewardPercentiles []float64) (*feeHistoryResult, error) {
	_, number, _, err := s.cli.Accepted(ctx)
	if err != nil {
		return nil, err
	}
	rewards := make([][]*hexutil.Big, 0)
	baseFee := make([]*hexutil.Big, 0)
	gasUsedRatio := make([]float64, 0)

	baseFee = append(baseFee, (*hexutil.Big)(big.NewInt(0)))
	gasUsedRatio = append(gasUsedRatio, 0)
	rewards = append(rewards, make([]*hexutil.Big, len(rewardPercentiles)))
	for i := 0; i < len(rewardPercentiles); i++ {
		rewards[0][i] = (*hexutil.Big)(big.NewInt(0))
	}

	return &feeHistoryResult{
		OldestBlock:  (*hexutil.Big)(new(big.Int).SetUint64(number)),
		Reward:       rewards,
		BaseFee:      baseFee,
		GasUsedRatio: gasUsedRatio,
	}, nil
}

func (s *API) EstimateGas(ctx context.Context, args eth.TransactionArgs, blockNrOrHash *eth_rpc.BlockNumberOrHash, overrides *eth.StateOverride) (hexutil.Uint64, error) {
	lotsOfGas := uint64(25000000)
	evmAddr := actions.ToEVMAddress(s.priv.Address)
	_, nonce, err := s.bcli.EvmAccount(ctx, evmAddr.Hex())
	if err != nil {
		return 0, err
	}
	to := args.To
	relayFor := common.HexToAddress(evmRelayAddress)
	if to != nil && *to == relayFor {
		to = &evmAddr
	}
	call := &actions.EvmCall{
		To:       to,
		GasLimit: lotsOfGas,
		Nonce:    nonce,
		Value:    new(big.Int),

		// Gas is priced at 0
		GasPrice:  new(big.Int),
		GasFeeCap: new(big.Int),
		GasTipCap: new(big.Int),
	}
	if args.Value != nil {
		call.Value = (*big.Int)(args.Value)
	}
	if args.Data != nil {
		call.Data = ([]byte)(*args.Data)
	}
	trace, err := s.bcli.TraceAction(ctx, brpc.TraceTxArgs{
		Action: *call,
		Actor:  s.priv.Address,
	})
	if err != nil {
		return 0, err
	}
	utils.Outf("{{yellow}}EstimateGas: %+v %+v{{/}}\n", args, trace)
	return hexutil.Uint64(trace.UsedGas), nil
}

func (s *API) Call(ctx context.Context, args eth.TransactionArgs, blockNrOrHash eth_rpc.BlockNumberOrHash, overrides *eth.StateOverride, blockOverrides *eth.BlockOverrides) (hexutil.Bytes, error) {
	lotsOfGas := uint64(25000000)
	evmAddr := actions.ToEVMAddress(s.priv.Address)
	_, nonce, err := s.bcli.EvmAccount(ctx, evmAddr.Hex())
	if err != nil {
		return nil, err
	}
	to := args.To
	relayFor := common.HexToAddress(evmRelayAddress)
	if to != nil && *to == relayFor {
		to = &evmAddr
	}
	call := &actions.EvmCall{
		To:       to,
		GasLimit: lotsOfGas,
		Nonce:    nonce,
		Value:    new(big.Int),

		// Gas is priced at 0
		GasPrice:  new(big.Int),
		GasFeeCap: new(big.Int),
		GasTipCap: new(big.Int),
	}
	if args.Value != nil {
		call.Value = (*big.Int)(args.Value)
	}
	if args.Data != nil {
		call.Data = ([]byte)(*args.Data)
	}
	if args.Gas != nil {
		call.GasLimit = uint64(*args.Gas)
	}
	utils.Outf("{{blue}}Call: %+v %+v{{/}}\n", call)
	trace, err := s.bcli.TraceAction(ctx, brpc.TraceTxArgs{
		Action: *call,
		Actor:  s.priv.Address,
	})
	if err != nil {
		return nil, err
	}
	utils.Outf("{{yellow}}Call: %+v %+v{{/}}\n", args, trace)
	return hexutil.Bytes(trace.Output), nil
}

func (s *API) SendRawTransaction(ctx context.Context, input hexutil.Bytes) (common.Hash, error) {
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(input); err != nil {
		return common.Hash{}, err
	}
	to := tx.To()
	relayFor := common.HexToAddress(evmRelayAddress)
	if to != nil && *to == relayFor {
		evmAddr := actions.ToEVMAddress(s.priv.Address)
		to = &evmAddr
	}
	call := &actions.EvmCall{
		To:       to,
		Nonce:    tx.Nonce(),
		Value:    tx.Value(),
		GasLimit: tx.Gas(),
		Data:     tx.Data(),

		// Gas is priced at 0
		GasPrice:  new(big.Int),
		GasFeeCap: new(big.Int),
		GasTipCap: new(big.Int),
	}
	trace, err := s.bcli.TraceAction(ctx, brpc.TraceTxArgs{
		Action: *call,
		Actor:  s.priv.Address,
	})
	if err != nil {
		return common.Hash{}, err
	}
	if !trace.Success {
		return common.Hash{}, fmt.Errorf("transaction failed: %v", trace.Error)
	}
	p := codec.NewReader(trace.StateKeys, len(trace.StateKeys))
	call.Keys, err = actions.UnmarshalKeys(p)
	if err != nil {
		return common.Hash{}, err
	}
	parser, err := s.bcli.Parser(ctx)
	if err != nil {
		return common.Hash{}, err
	}
	_, sentTx, _, err := s.cli.GenerateTransaction(ctx, parser, nil, call, s.factory)
	if err != nil {
		return common.Hash{}, err
	}
	if err := s.ws.RegisterTx(sentTx); err != nil {
		return common.Hash{}, err
	}
	utils.Outf("{{yellow}}Sent transaction: %s{{/}}\n", common.Hash(sentTx.ID()))
	if call.To == nil {
		s.registerContractCreation(sentTx.ID(), call.Nonce)
	}
	return common.Hash(sentTx.ID()), nil
}

func (s *API) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	found, success, timestamp, fee, err := s.bcli.Tx(ctx, ids.ID(hash))
	if err != nil {
		utils.Outf("{{red}}GetTransactionReceipt: %v %v{{/}}\n", hash, err)
		return nil, err
	}
	if !found {
		utils.Outf("{{yellow}}GetTransactionReceipt: %v not found{{/}}\n", hash)
		return nil, nil
	}
	var (
		successUint     hexutil.Uint
		contractAddress *common.Address
	)
	if success {
		successUint = hexutil.Uint(1)
		contractAddress = s.getContractCreation(ids.ID(hash))
	}

	blockHash := common.Hash{}
	fields := map[string]interface{}{
		"blockHash":         blockHash,
		"blockNumber":       (*hexutil.Big)(big.NewInt(1)),
		"transactionHash":   hash,
		"transactionIndex":  hexutil.Uint(0),
		"from":              actions.ToEVMAddress(s.priv.Address),
		"to":                common.Address{0x32},
		"gasUsed":           hexutil.Uint64(fee),
		"cumulativeGasUsed": hexutil.Uint64(fee),
		"contractAddress":   contractAddress,
		"logs":              []interface{}{},
		"logsBloom":         types.Bloom{},
		"status":            successUint,
		"blockTime":         timestamp / 1000,
		"type":              hexutil.Uint(0x2),
		"effectiveGasPrice": (*hexutil.Big)(big.NewInt(0)),
	}
	utils.Outf("{{yellow}}GetTransactionReceipt: %+v{{/}}\n", fields)
	return fields, nil
}

func (s *API) GetTransactionByHash(ctx context.Context, hash common.Hash) (*eth.RPCTransaction, error) {
	found, _, _, fee, err := s.bcli.Tx(ctx, ids.ID(hash))
	if err != nil {
		utils.Outf("{{red}}GetTransactionByHash: %v %v{{/}}\n", hash, err)
		return nil, err
	}
	if !found {
		utils.Outf("{{yellow}}GetTransactionByHash: %v not found{{/}}\n", hash)
		return nil, nil
	}
	blockHash := common.Hash{}
	to := common.Address{}
	fields := &eth.RPCTransaction{
		BlockHash:        &blockHash,
		BlockNumber:      (*hexutil.Big)(big.NewInt(1)),
		From:             actions.ToEVMAddress(s.priv.Address),
		Gas:              hexutil.Uint64(fee),
		GasPrice:         (*hexutil.Big)(big.NewInt(0)),
		GasFeeCap:        (*hexutil.Big)(big.NewInt(0)),
		GasTipCap:        (*hexutil.Big)(big.NewInt(0)),
		Hash:             hash,
		Input:            []byte{},
		To:               &to,
		TransactionIndex: new(hexutil.Uint64),
		Value:            (*hexutil.Big)(big.NewInt(0)),
		Type:             0x2,
		Accesses:         nil,
		ChainID:          s.ChainId(),
		V:                (*hexutil.Big)(big.NewInt(0)),
		R:                (*hexutil.Big)(big.NewInt(0)),
		S:                (*hexutil.Big)(big.NewInt(0)),
		YParity:          new(hexutil.Uint64),
	}
	utils.Outf("{{yellow}}GetTransactionByHash: %+v{{/}}\n", fields)
	return fields, nil
}

func (s *API) GetCode(ctx context.Context, address common.Address, blockNrOrHash eth_rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	code, err := s.bcli.EvmGetCode(ctx, address.Hex())
	utils.Outf("{{yellow}}GetCode: %s %s{{/}}\n", address.Hex(), hexutil.Bytes(code).String())
	return code, err
}

func LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the request body
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Println("Error reading request body:", err)
			return
		}
		// Restore the request body to its original state
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Log information about the request including the body
		fmt.Printf("Received %s request for %s from %s\n", r.Method, r.URL.Path, r.RemoteAddr)
		fmt.Println("Request body:", string(body))

		// Call the next handler in the chain
		next.ServeHTTP(w, r)
	})
}

var evmRelayCmd = &cobra.Command{
	Use: "evm-relay",
	RunE: func(*cobra.Command, []string) error {
		_, priv, factory, cli, bcli, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		noDeadline := time.Duration(0)
		server := eth_rpc.NewServer(noDeadline)
		config := params.SubnetEVMDefaultChainConfig
		api := &API{
			chainConfig: params.SubnetEVMDefaultChainConfig,
			priv:        priv,
			factory:     factory,
			cli:         cli,
			bcli:        bcli,
			ws:          ws,
		}
		err = server.RegisterName("eth", api)
		if err != nil {
			return err
		}
		chainID := config.ChainID.Uint64()
		err = server.RegisterName("net", eth.NewNetAPI(chainID))
		if err != nil {
			return err
		}

		addr := net.JoinHostPort("localhost", fmt.Sprintf("%d", evmRelayPort))
		http.Handle("/evm-relay/rpc", LogRequests(server))

		timeout := 30 * time.Second
		sever := &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: timeout,
		}

		go func() {
			utils.Outf("{{green}}Listening on %s{{/}}\n", addr)
			err := sever.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				panic(err)
			}
		}()

		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
		<-interrupt

		if err := sever.Shutdown(context.Background()); err != nil {
			return err
		}

		return nil
	},
}

func makeEvmRelayCmd() *cobra.Command {
	evmRelayCmd.Flags().IntVar(&evmRelayPort, "port", 11000, "port to listen on")
	evmRelayCmd.Flags().StringVar(&evmRelayAddress, "address", "", "address to relay from")
	return evmRelayCmd
}

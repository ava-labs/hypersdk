// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/actions"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/consts"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/requester"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

const balanceCheckInterval = 500 * time.Millisecond

type JSONRPCClient struct {
	requester *requester.EndpointRequester
	g         *genesis.DefaultGenesis
}

// NewJSONRPCClient creates a new client object.
func NewJSONRPCClient(uri string) *JSONRPCClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += JSONRPCEndpoint
	req := requester.New(uri, consts.Name)
	return &JSONRPCClient{req, nil}
}

func (cli *JSONRPCClient) Genesis(ctx context.Context) (*genesis.DefaultGenesis, error) {
	if cli.g != nil {
		return cli.g, nil
	}

	resp := new(GenesisReply)
	err := cli.requester.SendRequest(
		ctx,
		"genesis",
		nil,
		resp,
	)
	if err != nil {
		return nil, err
	}
	cli.g = resp.Genesis
	return resp.Genesis, nil
}

func (cli *JSONRPCClient) Balance(ctx context.Context, addr codec.Address) (uint64, error) {
	resp := new(BalanceReply)
	err := cli.requester.SendRequest(
		ctx,
		"balance",
		&BalanceArgs{
			Address: addr,
		},
		resp,
	)
	return resp.Amount, err
}

func (cli *JSONRPCClient) WaitForBalance(
	ctx context.Context,
	addr codec.Address,
	min uint64,
) error {
	return jsonrpc.Wait(ctx, balanceCheckInterval, func(ctx context.Context) (bool, error) {
		balance, err := cli.Balance(ctx, addr)
		if err != nil {
			return false, err
		}
		shouldExit := balance >= min
		if !shouldExit {
			utils.Outf(
				"{{yellow}}waiting for %s balance: %s{{/}}\n",
				utils.FormatBalance(min),
				addr,
			)
		}
		return shouldExit, nil
	})
}

func (cli *JSONRPCClient) Parser(ctx context.Context) (chain.Parser, error) {
	g, err := cli.Genesis(ctx)
	if err != nil {
		return nil, err
	}
	registry, err := newRegistry()
	if err != nil {
		return nil, err
	}
	return NewParser(g, registry), nil
}

var _ chain.Parser = (*Parser)(nil)

type Parser struct {
	genesis  *genesis.DefaultGenesis
	registry chain.Registry
}

func (p *Parser) Rules(_ int64) chain.Rules {
	return p.genesis.Rules
}

func (p *Parser) ActionRegistry() chain.ActionRegistry {
	return p.registry.ActionRegistry()
}

func (p *Parser) AuthRegistry() chain.AuthRegistry {
	return p.registry.AuthRegistry()
}

func (p *Parser) OutputRegistry() chain.OutputRegistry {
	return p.registry.OutputRegistry()
}

func (*Parser) StateManager() chain.StateManager {
	return &storage.StateManager{}
}

func NewParser(genesis *genesis.DefaultGenesis, registry chain.Registry) chain.Parser {
	return &Parser{genesis: genesis, registry: registry}
}

// Used as a lambda function for creating ExternalSubscriberServer parser
func CreateParser(genesisBytes []byte) (chain.Parser, error) {
	var genesis genesis.DefaultGenesis
	if err := json.Unmarshal(genesisBytes, &genesis); err != nil {
		return nil, err
	}
	registry, err := newRegistry()
	if err != nil {
		return nil, err
	}
	return NewParser(&genesis, registry), nil
}

func (cli *JSONRPCClient) Simulate(ctx context.Context, callTx actions.Call, actor codec.Address) (state.Keys, uint64, error) {
	resp := new(SimulateCallTxReply)
	err := cli.requester.SendRequest(
		ctx,
		"simulateCallContractTx",
		&SimulateCallTxArgs{CallTx: callTx, Actor: actor},
		resp,
	)
	if err != nil {
		return nil, 0, err
	}
	result := state.Keys{}
	for _, entry := range resp.StateKeys {
		hexBytes, err := hex.DecodeString(entry.HexKey)
		if err != nil {
			return nil, 0, err
		}

		result.Add(string(hexBytes), state.Permissions(entry.Permissions))
	}
	return result, resp.FuelConsumed, nil
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"strings"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/registry"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/requester"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
)

type JSONRPCClient struct {
	requester *requester.EndpointRequester
	g         *vm.BaseGenesis
	rules     *vm.BaseRules
	networkID uint32
	chainID   ids.ID
}

// NewJSONRPCClient creates a new client object.
func NewJSONRPCClient(uri string) *JSONRPCClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += JSONRPCEndpoint
	req := requester.New(uri, consts.Name)
	return &JSONRPCClient{req, nil, nil, 0, ids.Empty}
}

func (cli *JSONRPCClient) Genesis(ctx context.Context) (*vm.BaseGenesis, error) {
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

func (cli *JSONRPCClient) Rules(ctx context.Context) (*vm.BaseRules, error) {
	if cli.rules != nil {
		return cli.rules, nil
	}

	resp := new(RulesReply)
	err := cli.requester.SendRequest(
		ctx,
		"rules",
		nil,
		resp,
	)
	if err != nil {
		return nil, err
	}
	cli.rules = resp.Rules
	return resp.Rules, nil
}

func (cli *JSONRPCClient) Tx(ctx context.Context, id ids.ID) (bool, bool, int64, uint64, error) {
	resp := new(TxReply)
	err := cli.requester.SendRequest(
		ctx,
		"tx",
		&TxArgs{TxID: id},
		resp,
	)
	switch {
	// We use string parsing here because the JSON-RPC library we use may not
	// allows us to perform errors.Is.
	case err != nil && strings.Contains(err.Error(), ErrTxNotFound.Error()):
		return false, false, -1, 0, nil
	case err != nil:
		return false, false, -1, 0, err
	}
	return true, resp.Success, resp.Timestamp, resp.Fee, nil
}

func (cli *JSONRPCClient) Balance(ctx context.Context, addr string) (uint64, error) {
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
	addr string,
	min uint64,
) error {
	return rpc.Wait(ctx, func(ctx context.Context) (bool, error) {
		balance, err := cli.Balance(ctx, addr)
		if err != nil {
			return false, err
		}
		shouldExit := balance >= min
		if !shouldExit {
			utils.Outf(
				"{{yellow}}waiting for %s balance: %s{{/}}\n",
				utils.FormatBalance(min, consts.Decimals),
				addr,
			)
		}
		return shouldExit, nil
	})
}

func (cli *JSONRPCClient) WaitForTransaction(ctx context.Context, txID ids.ID) (bool, uint64, error) {
	var success bool
	var fee uint64
	if err := rpc.Wait(ctx, func(ctx context.Context) (bool, error) {
		found, isuccess, _, ifee, err := cli.Tx(ctx, txID)
		if err != nil {
			return false, err
		}
		success = isuccess
		fee = ifee
		return found, nil
	}); err != nil {
		return false, 0, err
	}
	return success, fee, nil
}

var _ chain.Parser = (*Parser)(nil)

type Parser struct {
	genesis *vm.BaseGenesis
	rules   vm.RuleFactory
}

func (p *Parser) Rules(t int64) chain.Rules {
	return p.rules.GetRules(t)
}

func (*Parser) Registry() (chain.ActionRegistry, chain.AuthRegistry) {
	return registry.Action, registry.Auth
}

func (*Parser) StateManager() chain.StateManager {
	return &storage.StateManager{}
}

func (cli *JSONRPCClient) Parser(ctx context.Context) (chain.Parser, error) {
	g, err := cli.Genesis(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := cli.Rules(ctx)
	if err != nil {
		return nil, err
	}
	return &Parser{genesis: g, rules: &vm.UnchangingRuleFactory{UnchangingRules: rules}}, nil
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package jsonrpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/requester"
	"github.com/ava-labs/hypersdk/utils"
)

const unitPricesCacheRefresh = 10 * time.Second

type JSONRPCClient struct {
	requester *requester.EndpointRequester

	networkID uint32
	subnetID  ids.ID
	chainID   ids.ID

	lastUnitPrices time.Time
	unitPrices     fees.Dimensions
}

func NewJSONRPCClient(uri string) *JSONRPCClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += Endpoint
	req := requester.New(uri, api.Name)
	return &JSONRPCClient{requester: req}
}

func (cli *JSONRPCClient) Ping(ctx context.Context) (bool, error) {
	resp := new(PingReply)
	err := cli.requester.SendRequest(ctx,
		"ping",
		nil,
		resp,
	)
	return resp.Success, err
}

func (cli *JSONRPCClient) Network(ctx context.Context) (networkID uint32, subnetID ids.ID, chainID ids.ID, err error) {
	if cli.chainID != ids.Empty {
		return cli.networkID, cli.subnetID, cli.chainID, nil
	}

	resp := new(NetworkReply)
	err = cli.requester.SendRequest(
		ctx,
		"network",
		nil,
		resp,
	)
	if err != nil {
		return 0, ids.Empty, ids.Empty, err
	}
	cli.networkID = resp.NetworkID
	cli.subnetID = resp.SubnetID
	cli.chainID = resp.ChainID
	return resp.NetworkID, resp.SubnetID, resp.ChainID, nil
}

func (cli *JSONRPCClient) Accepted(ctx context.Context) (ids.ID, uint64, int64, error) {
	resp := new(LastAcceptedReply)
	err := cli.requester.SendRequest(
		ctx,
		"lastAccepted",
		nil,
		resp,
	)
	return resp.BlockID, resp.Height, resp.Timestamp, err
}

func (cli *JSONRPCClient) UnitPrices(ctx context.Context, useCache bool) (fees.Dimensions, error) {
	if useCache && time.Since(cli.lastUnitPrices) < unitPricesCacheRefresh {
		return cli.unitPrices, nil
	}

	resp := new(UnitPricesReply)
	err := cli.requester.SendRequest(
		ctx,
		"unitPrices",
		nil,
		resp,
	)
	if err != nil {
		return fees.Dimensions{}, err
	}
	cli.unitPrices = resp.UnitPrices
	// We update the time last in case there are concurrent requests being
	// processed (we don't want them to get an inconsistent view).
	cli.lastUnitPrices = time.Now()
	return resp.UnitPrices, nil
}

func (cli *JSONRPCClient) SubmitTx(ctx context.Context, d []byte) (ids.ID, error) {
	resp := new(SubmitTxReply)
	err := cli.requester.SendRequest(
		ctx,
		"submitTx",
		&SubmitTxArgs{Tx: d},
		resp,
	)
	return resp.TxID, err
}

type Modifier interface {
	Base(*chain.Base)
}

func (cli *JSONRPCClient) GenerateTransaction(
	ctx context.Context,
	parser chain.Parser,
	actions []chain.Action,
	authFactory chain.AuthFactory,
	modifiers ...Modifier,
) (func(context.Context) error, *chain.Transaction, uint64, error) {
	// Get latest fee info
	unitPrices, err := cli.UnitPrices(ctx, true)
	if err != nil {
		return nil, nil, 0, err
	}

	units, err := chain.EstimateUnits(parser.Rules(time.Now().UnixMilli()), actions, authFactory)
	if err != nil {
		return nil, nil, 0, err
	}
	maxFee, err := fees.MulSum(unitPrices, units)
	if err != nil {
		return nil, nil, 0, err
	}
	f, tx, err := cli.GenerateTransactionManual(parser, actions, authFactory, maxFee, modifiers...)
	if err != nil {
		return nil, nil, 0, err
	}
	return f, tx, maxFee, nil
}

func (cli *JSONRPCClient) GenerateTransactionManual(
	parser chain.Parser,
	actions []chain.Action,
	authFactory chain.AuthFactory,
	maxFee uint64,
	modifiers ...Modifier,
) (func(context.Context) error, *chain.Transaction, error) {
	// Construct transaction
	now := time.Now().UnixMilli()
	rules := parser.Rules(now)
	base := &chain.Base{
		Timestamp: utils.UnixRMilli(now, rules.GetValidityWindow()),
		ChainID:   rules.GetChainID(),
		MaxFee:    maxFee,
	}

	// Modify gathered data
	for _, m := range modifiers {
		m.Base(base)
	}

	// Build transaction
	actionCodec, authCodec := parser.ActionCodec(), parser.AuthCodec()
	tx := chain.NewTx(base, actions)
	tx, err := tx.Sign(authFactory, actionCodec, authCodec)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: failed to sign transaction", err)
	}

	// Return max fee and transaction for issuance
	return func(ictx context.Context) error {
		_, err := cli.SubmitTx(ictx, tx.Bytes())
		return err
	}, tx, nil
}

func (cli *JSONRPCClient) GetABI(ctx context.Context) (abi.ABI, error) {
	resp := new(GetABIReply)
	err := cli.requester.SendRequest(
		ctx,
		"getABI",
		nil,
		resp,
	)
	return resp.ABI, err
}

func (cli *JSONRPCClient) ExecuteActions(ctx context.Context, actor codec.Address, actions []chain.Action) ([][]byte, error) {
	actionsMarshaled := make([][]byte, 0, len(actions))
	for _, action := range actions {
		actionBytes, err := chain.MarshalTyped(action)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal action: %w", err)
		}
		actionsMarshaled = append(actionsMarshaled, actionBytes)
	}

	args := &ExecuteActionArgs{
		Actor:   actor,
		Actions: actionsMarshaled,
	}

	resp := new(ExecuteActionReply)
	err := cli.requester.SendRequest(
		ctx,
		"executeActions",
		args,
		resp,
	)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("failed to execute action: %s", resp.Error)
	}

	return resp.Outputs, nil
}

func Wait(ctx context.Context, interval time.Duration, check func(ctx context.Context) (bool, error)) error {
	for ctx.Err() == nil {
		exit, err := check(ctx)
		if err != nil {
			return err
		}
		if exit {
			return nil
		}
		time.Sleep(interval)
	}
	return ctx.Err()
}

func (cli *JSONRPCClient) SimulateActions(ctx context.Context, actions chain.Actions, actor codec.Address) ([]SimulateActionResult, error) {
	args := &SimulatActionsArgs{
		Actor: actor,
	}

	for _, action := range actions {
		marshaledAction, err := chain.MarshalTyped(action)
		if err != nil {
			return nil, err
		}
		args.Actions = append(args.Actions, marshaledAction)
	}

	resp := new(SimulateActionsReply)
	err := cli.requester.SendRequest(
		ctx,
		"simulateActions",
		args,
		resp,
	)
	if err != nil {
		return nil, err
	}

	return resp.ActionResults, nil
}

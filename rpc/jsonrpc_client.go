// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/requester"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	unitPricesCacheRefresh = 10 * time.Second
	waitSleep              = 500 * time.Millisecond
)

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
	uri += JSONRPCEndpoint
	req := requester.New(uri, Name)
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

func (cli *JSONRPCClient) Network(ctx context.Context) (uint32, ids.ID, ids.ID, error) {
	if cli.chainID != ids.Empty {
		return cli.networkID, cli.subnetID, cli.chainID, nil
	}

	resp := new(NetworkReply)
	err := cli.requester.SendRequest(
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
	action chain.Action,
	authFactory chain.AuthFactory,
	modifiers ...Modifier,
) (func(context.Context) error, *chain.Transaction, uint64, error) {
	// Get latest fee info
	unitPrices, err := cli.UnitPrices(ctx, true)
	if err != nil {
		return nil, nil, 0, err
	}

	maxUnits, err := chain.EstimateMaxUnits(parser.Rules(time.Now().UnixMilli()), action, authFactory)
	if err != nil {
		return nil, nil, 0, err
	}
	maxFee, err := fees.MulSum(unitPrices, maxUnits)
	if err != nil {
		return nil, nil, 0, err
	}
	f, tx, err := cli.GenerateTransactionManual(parser, action, authFactory, maxFee, modifiers...)
	if err != nil {
		return nil, nil, 0, err
	}
	return f, tx, maxFee, nil
}

func (cli *JSONRPCClient) GenerateTransactionManual(
	parser chain.Parser,
	action chain.Action,
	authFactory chain.AuthFactory,
	maxFee uint64,
	modifiers ...Modifier,
) (func(context.Context) error, *chain.Transaction, error) {
	// Construct transaction
	now := time.Now().UnixMilli()
	rules := parser.Rules(now)
	base := &chain.Base{
		Timestamp: utils.UnixRMilli(now, rules.GetValidityWindow()),
		ChainID:   rules.ChainID(),
		MaxFee:    maxFee,
	}

	// Modify gathered data
	for _, m := range modifiers {
		m.Base(base)
	}

	// Build transaction
	actionRegistry, authRegistry := parser.Registry()
	tx := chain.NewTx(base, action)
	tx, err := tx.Sign(authFactory, actionRegistry, authRegistry)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: failed to sign transaction", err)
	}

	// Return max fee and transaction for issuance
	return func(ictx context.Context) error {
		_, err := cli.SubmitTx(ictx, tx.Bytes())
		return err
	}, tx, nil
}

func Wait(ctx context.Context, check func(ctx context.Context) (bool, error)) error {
	for ctx.Err() == nil {
		exit, err := check(ctx)
		if err != nil {
			return err
		}
		if exit {
			return nil
		}
		time.Sleep(waitSleep)
	}
	return ctx.Err()
}

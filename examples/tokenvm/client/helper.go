// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package client

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/client"
	"github.com/ava-labs/hypersdk/utils"
)

func (cli *Client) GenerateTransaction(
	ctx context.Context,
	action chain.Action,
	factory chain.AuthFactory,
	modifiers ...client.Modifier,
) (func(context.Context) error, *chain.Transaction, uint64, error) {
	// Gather chain metadata
	g, err := cli.Genesis(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	_, _, chainID, err := cli.Network(ctx) // TODO: store in object to fetch less frequently
	if err != nil {
		return nil, nil, 0, err
	}
	return cli.Client.GenerateTransaction(
		ctx,
		&Parser{chainID, g},
		action,
		factory,
		modifiers...)
}

func (cli *Client) WaitForBalance(
	ctx context.Context,
	addr string,
	asset ids.ID,
	min uint64,
) error {
	return client.Wait(ctx, func(ctx context.Context) (bool, error) {
		balance, err := cli.Balance(ctx, addr, asset)
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

func (cli *Client) WaitForTransaction(ctx context.Context, txID ids.ID) (bool, error) {
	var success bool
	if err := client.Wait(ctx, func(ctx context.Context) (bool, error) {
		found, isuccess, _, err := cli.Tx(ctx, txID)
		if err != nil {
			return false, err
		}
		success = isuccess
		return found, nil
	}); err != nil {
		return false, err
	}
	return success, nil
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package client

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/hypersdk/chain"
)

const (
	waitSleep = 3 * time.Second
)

type Modifier interface {
	Base(*chain.Base)
}

func (cli *Client) GenerateTransaction(
	ctx context.Context,
	parser chain.Parser,
	action chain.Action,
	authFactory chain.AuthFactory,
	modifiers ...Modifier,
) (func(context.Context) error, *chain.Transaction, uint64, error) {
	// Get latest fee info
	unitPrice, _, err := cli.SuggestedRawFee(ctx)
	if err != nil {
		return nil, nil, 0, err
	}

	// Construct transaction
	now := time.Now().Unix()
	rules := parser.Rules(now)
	base := &chain.Base{
		Timestamp: now + rules.GetValidityWindow(),
		ChainID:   rules.GetChainID(),
		UnitPrice: unitPrice, // never pay blockCost
	}

	// Modify gathered data
	for _, m := range modifiers {
		m.Base(base)
	}

	// Build transaction
	tx := chain.NewTx(base, action)
	if err := tx.Sign(authFactory); err != nil {
		return nil, nil, 0, fmt.Errorf("%w: failed to sign transaction", err)
	}
	actionRegistry, authRegistry := parser.Registry()
	// TODO: init?
	if _, err := tx.Init(ctx, actionRegistry, authRegistry); err != nil {
		return nil, nil, 0, fmt.Errorf("%w: failed to init transaction", err)
	}

	// Return max fee and transaction for issuance
	return func(ictx context.Context) error {
		_, err := cli.SubmitTx(ictx, tx.Bytes())
		return err
	}, tx, unitPrice * tx.MaxUnits(rules), nil
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

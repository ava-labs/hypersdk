// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/load"
)

var (
	ErrTxGeneratorFundsExhausted = errors.New("tx generator funds exhausted")

	_ load.Issuer[*chain.Transaction] = (*Issuer)(nil)
)

type Issuer struct {
	authFactory chain.AuthFactory
	currBalance uint64
	ruleFactory chain.RuleFactory
	unitPrices  fees.Dimensions

	client  *ws.WebSocketClient
	tracker Tracker
}

func NewIssuer(
	authFactory chain.AuthFactory,
	ruleFactory chain.RuleFactory,
	currBalance uint64,
	unitPrices fees.Dimensions,
	client *ws.WebSocketClient,
	tracker Tracker,
) *Issuer {
	return &Issuer{
		authFactory: authFactory,
		ruleFactory: ruleFactory,
		currBalance: currBalance,
		unitPrices:  unitPrices,
		client:      client,
		tracker:     tracker,
	}
}

func (i *Issuer) GenerateAndIssueTx(_ context.Context) (*chain.Transaction, error) {
	tx, err := chain.GenerateTransaction(
		i.ruleFactory,
		i.unitPrices,
		time.Now().UnixMilli(),
		[]chain.Action{
			&actions.Transfer{
				To:    i.authFactory.Address(),
				Value: 1,
				Memo:  binary.BigEndian.AppendUint64(nil, i.currBalance),
			},
		},
		i.authFactory,
	)
	if err != nil {
		return nil, err
	}
	if tx.MaxFee() > i.currBalance {
		return nil, ErrTxGeneratorFundsExhausted
	}
	i.currBalance -= tx.MaxFee()

	if err := i.client.RegisterTx(tx); err != nil {
		return nil, err
	}
	i.tracker.Issue(tx.GetID())
	return tx, nil
}

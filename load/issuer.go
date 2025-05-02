// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
)

var _ Issuer[*chain.Transaction] = (*DefaultIssuer)(nil)

type DefaultIssuer struct {
	client  *ws.WebSocketClient
	tracker Tracker[ids.ID]
}

func NewDefaultIssuer(client *ws.WebSocketClient, tracker Tracker[ids.ID]) *DefaultIssuer {
	return &DefaultIssuer{
		client:  client,
		tracker: tracker,
	}
}

func (i *DefaultIssuer) IssueTx(_ context.Context, tx *chain.Transaction) error {
	if err := i.client.RegisterTx(tx); err != nil {
		return err
	}
	// Update tracker
	i.tracker.Issue(tx.GetID())
	return nil
}

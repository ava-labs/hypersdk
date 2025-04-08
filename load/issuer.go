// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"errors"
	"sync"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/pubsub"
)

var (
	_ Issuer[*chain.Transaction] = (*DefaultIssuer)(nil)

	ErrIssuedAlreadyStopped = errors.New("issuer already stopped")
)

type DefaultIssuer struct {
	client  *ws.WebSocketClient
	tracker Tracker[ids.ID]

	lock        sync.RWMutex
	issuedTxs   uint64
	receivedTxs uint64
	stopped     bool
}

func NewDefaultIssuer(uri string, tracker Tracker[ids.ID]) (*DefaultIssuer, error) {
	client, err := ws.NewWebSocketClient(
		uri,
		ws.DefaultHandshakeTimeout,
		pubsub.MaxPendingMessages,
		pubsub.MaxReadMessageSize,
	)
	if err != nil {
		return nil, err
	}

	return &DefaultIssuer{
		client:  client,
		tracker: tracker,
	}, nil
}

func (i *DefaultIssuer) Listen(ctx context.Context) error {
	defer i.client.Close()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		txID, result, err := i.client.ListenTx(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		// accepted txs have a non-nil result
		if result != nil {
			i.tracker.ObserveConfirmed(txID)
		} else {
			i.tracker.ObserveFailed(txID)
		}

		i.incrementReceivedTxs()
		if i.isFinished() {
			return nil
		}
	}
}

func (i *DefaultIssuer) Stop() {
	i.lock.Lock()
	defer i.lock.Unlock()

	i.stopped = true
}

func (i *DefaultIssuer) IssueTx(_ context.Context, tx *chain.Transaction) error {
	i.lock.Lock()
	defer i.lock.Unlock()

	if i.stopped {
		return ErrIssuedAlreadyStopped
	}

	if err := i.client.RegisterTx(tx); err != nil {
		return err
	}
	// Update tracker
	i.tracker.Issue(tx.GetID())
	i.issuedTxs++
	return nil
}

func (i *DefaultIssuer) incrementReceivedTxs() {
	i.lock.Lock()
	defer i.lock.Unlock()

	i.receivedTxs++
}

// isFinished returns true if all transactions have been heard
func (i *DefaultIssuer) isFinished() bool {
	i.lock.RLock()
	defer i.lock.RUnlock()

	return i.stopped && i.issuedTxs == i.receivedTxs
}

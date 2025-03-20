// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/pubsub"
)

var ErrIssuedAlreadyStopped = errors.New("issuer already stopped")

type DefaultIssuer struct {
	client  *ws.WebSocketClient
	tracker Tracker

	lock     sync.Mutex
	numOfTxs uint64
	heardTxs uint64
	stopped  bool
}

func NewDefaultIssuer(uri string, tracker Tracker) (*DefaultIssuer, error) {
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
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		txID, result, err := i.client.ListenTx(ctx)
		if err != nil {
			return err
		}
		// accepted txs have a non-nil result
		if result != nil {
			i.tracker.ObserveConfirmed(txID)
		} else {
			i.tracker.ObserveFailed(txID)
		}

		i.lock.Lock()
		i.heardTxs++
		// if we've heard the status of all of our transactions, return nil
		if i.stopped && i.numOfTxs == i.heardTxs {
			i.lock.Unlock()
			return nil
		}
		i.lock.Unlock()
	}
}

// Stop notifies the DefaultIssuer that the current number of txs it has sent is final
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
	now := time.Now()
	i.tracker.Issue(tx.GetID(), now)
	i.numOfTxs++
	return nil
}

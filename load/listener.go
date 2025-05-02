// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api/ws"
)

var _ Listener = (*DefaultListener)(nil)

type DefaultListener struct {
	client  *ws.WebSocketClient
	tracker Tracker[ids.ID]

	lock        sync.RWMutex
	issuedTxs   uint64
	receivedTxs uint64
	stopped     bool
}

func NewDefaultListener(client *ws.WebSocketClient, tracker Tracker[ids.ID]) *DefaultListener {
	return &DefaultListener{
		client:  client,
		tracker: tracker,
	}
}

func (l *DefaultListener) Listen(ctx context.Context) (err error) {
	defer func() {
		closeErr := l.client.Close()
		if closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	for !l.isFinished() {
		txID, result, err := l.client.ListenTx(ctx)
		if ctx.Err() != nil {
			return nil
		} else if err != nil {
			return err
		}
		// accepted txs have a non-nil result
		if result != nil {
			l.tracker.ObserveConfirmed(txID)
		} else {
			l.tracker.ObserveFailed(txID)
		}

		l.incrementReceivedTxs()
	}
	return nil
}

func (l *DefaultListener) Stop(issued uint64) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.issuedTxs = issued
	l.stopped = true
}

// isFinished returns true if all transactions have been heard
func (l *DefaultListener) isFinished() bool {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return l.stopped && l.issuedTxs == l.receivedTxs
}

func (l *DefaultListener) incrementReceivedTxs() {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.receivedTxs++
}

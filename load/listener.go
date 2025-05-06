// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
)

var _ Listener[*chain.Transaction] = (*DefaultListener)(nil)

type DefaultListener struct {
	client  *ws.WebSocketClient
	tracker Tracker[ids.ID]

	lock          sync.RWMutex
	issuedTxs     uint64
	lastIssued    bool
	receivedTxs   uint64
	inFlightTxIDs set.Set[ids.ID]
}

// NewDefaultListener creates a new [DefaultListener] instance.
func NewDefaultListener(client *ws.WebSocketClient, tracker Tracker[ids.ID]) *DefaultListener {
	return &DefaultListener{
		client:        client,
		tracker:       tracker,
		inFlightTxIDs: set.NewSet[ids.ID](1),
	}
}

func (l *DefaultListener) Listen(ctx context.Context) (err error) {
	defer func() {
		closeErr := l.client.Close()
		if closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	defer l.markRemainingAsFailed()

	for {
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

		l.lock.Lock()
		delete(l.inFlightTxIDs, txID)
		l.receivedTxs++
		if l.lastIssued && len(l.inFlightTxIDs) == 0 {
			l.lock.Unlock()
			break
		}
		l.lock.Unlock()
	}
	return nil
}

func (l *DefaultListener) RegisterIssued(tx *chain.Transaction, last bool) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.lastIssued = last
	l.issuedTxs++
	l.inFlightTxIDs.Add(tx.GetID())
}

func (l *DefaultListener) markRemainingAsFailed() {
	l.lock.RLock()
	defer l.lock.RUnlock()
	for txID := range l.inFlightTxIDs {
		l.tracker.ObserveFailed(txID)
	}
	l.inFlightTxIDs.Clear()
}

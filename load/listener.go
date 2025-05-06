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
	client   *ws.WebSocketClient
	tracker  Tracker[ids.ID]
	txTarget uint64

	lock          sync.RWMutex
	issuedTxs     uint64
	receivedTxs   uint64
	inFlightTxIDs set.Set[ids.ID]
}

// NewDefaultListener creates a new DefaultListener instance.
// Set txTarget to 0 to never stop the [DefaultListener.Listen] method unless the context is cancelled.
func NewDefaultListener(client *ws.WebSocketClient, tracker Tracker[ids.ID], txTarget uint64) *DefaultListener {
	return &DefaultListener{
		client:        client,
		tracker:       tracker,
		txTarget:      txTarget,
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
		if l.txTarget > 0 &&
			l.issuedTxs == l.txTarget &&
			l.issuedTxs == l.receivedTxs &&
			len(l.inFlightTxIDs) == 0 {
			l.lock.Unlock()
			break
		}
		l.lock.Unlock()
	}
	return nil
}

func (l *DefaultListener) RegisterIssued(tx *chain.Transaction) {
	l.lock.Lock()
	defer l.lock.Unlock()

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

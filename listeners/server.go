// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package listeners

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/pubsub"
)

type Listeners struct {
	txL         sync.Mutex
	txListeners map[ids.ID]*pubsub.Connections
	expiringTxs *emap.EMap[*chain.Transaction] // ensures all tx listeners are eventually responded to
	s           *pubsub.Server
}

func New() *Listeners {
	return &Listeners{
		txListeners: map[ids.ID]*pubsub.Connections{},
		expiringTxs: emap.NewEMap[*chain.Transaction](),
	}
}

func (w *Listeners) SetServer(s *pubsub.Server) {
	w.s = s
}

// Note: no need to have a tx listener removal, this will happen when all
// submitted transactions are cleared.
func (w *Listeners) AddTxListener(tx *chain.Transaction, c *pubsub.Connection) {
	w.txL.Lock()
	defer w.txL.Unlock()

	txID := tx.ID()
	if _, ok := w.txListeners[txID]; !ok {
		w.txListeners[txID] = pubsub.NewConnections()
	}
	w.txListeners[txID].Add(c)
	w.expiringTxs.Add([]*chain.Transaction{tx})
}

// If never possible for a tx to enter mempool, call this
func (w *Listeners) RemoveTx(txID ids.ID, err error) {
	w.txL.Lock()
	defer w.txL.Unlock()

	w.removeTx(txID, err)
}

func (w *Listeners) removeTx(txID ids.ID, err error) {
	listeners, ok := w.txListeners[txID]
	if !ok {
		return
	}
	p := codec.NewWriter(consts.MaxInt)
	PackRemovedTxMessage(p, txID, err)
	w.s.Publish(txID[:], listeners)
	delete(w.txListeners, txID)
	// [expiringTxs] will be cleared eventually (does not support removal)
}

func (w *Listeners) SetMinTx(t int64) {
	w.txL.Lock()
	defer w.txL.Unlock()

	expired := w.expiringTxs.SetMin(t)
	for _, id := range expired {
		w.removeTx(id, ErrExpired)
	}
}

func (w *Listeners) AcceptBlock(b *chain.StatelessBlock) {
	p := codec.NewWriter(consts.MaxInt)
	PackBlockMessageBytes(b, p)
	// Publish accepted block to all block listeners
	w.s.Publish(p.Bytes(), w.s.Connections())
	w.txL.Lock()
	defer w.txL.Unlock()
	results := b.Results()
	for i, tx := range b.Txs {
		p := codec.NewWriter(consts.MaxInt)
		txID := tx.ID()
		listeners, ok := w.txListeners[txID]
		if !ok {
			continue
		}
		// Publish to tx listener
		PackAcceptedTxMessage(p, txID, results[i])
		w.s.Publish(
			p.Bytes(),
			listeners,
		)
		delete(w.txListeners, txID)
		// [expiringTxs] will be cleared eventually (does not support removal)
	}
}

func (w *Listeners) ServerCallback(vm VM) pubsub.Callback {
	var (
		actionRegistry, authRegistry = vm.Registry()
		tracer                       = vm.Tracer()
		log                          = vm.Logger()
	)
	return func(msgBytes []byte, c *pubsub.Connection) {
		// TODO: support multiple callbacks
		ctx, span := tracer.Start(context.Background(), "listener callback")
		defer span.End()

		// Unmarshal TX
		p := codec.NewReader(msgBytes, chain.NetworkSizeLimit) // will likely be much smaller
		tx, err := chain.UnmarshalTx(p, actionRegistry, authRegistry)
		if err != nil {
			log.Error("failed to unmarshal tx",
				zap.Int("len", len(msgBytes)),
				zap.Error(err),
			)
			return
		}

		// Verify tx
		sigVerify := tx.AuthAsyncVerify()
		if err := sigVerify(); err != nil {
			log.Error("failed to verify sig",
				zap.Error(err),
			)
			return
		}
		w.AddTxListener(tx, c)

		// Submit will remove from [txWaiters] if it is not added
		txID := tx.ID()
		if err := vm.Submit(ctx, false, []*chain.Transaction{tx})[0]; err != nil {
			log.Error("failed to submit tx",
				zap.Stringer("txID", txID),
				zap.Error(err),
			)
			return
		}
		log.Debug("submitted tx", zap.Stringer("id", txID))
	}
}

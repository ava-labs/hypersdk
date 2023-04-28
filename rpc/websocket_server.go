// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/pubsub"
)

type WebSocketServer struct {
	txL               sync.Mutex
	txWebSocketServer map[ids.ID]*pubsub.Connections
	expiringTxs       *emap.EMap[*chain.Transaction] // ensures all tx listeners are eventually responded to
	s                 *pubsub.Server
}

func NewWebSocketServer() *WebSocketServer {
	return &WebSocketServer{
		txWebSocketServer: map[ids.ID]*pubsub.Connections{},
		expiringTxs:       emap.NewEMap[*chain.Transaction](),
	}
}

func (w *WebSocketServer) SetBackend(s *pubsub.Server) {
	w.s = s
}

// Note: no need to have a tx listener removal, this will happen when all
// submitted transactions are cleared.
func (w *WebSocketServer) AddTxListener(tx *chain.Transaction, c *pubsub.Connection) {
	w.txL.Lock()
	defer w.txL.Unlock()

	txID := tx.ID()
	if _, ok := w.txWebSocketServer[txID]; !ok {
		w.txWebSocketServer[txID] = pubsub.NewConnections()
	}
	w.txWebSocketServer[txID].Add(c)
	w.expiringTxs.Add([]*chain.Transaction{tx})
}

// If never possible for a tx to enter mempool, call this
func (w *WebSocketServer) RemoveTx(txID ids.ID, err error) {
	w.txL.Lock()
	defer w.txL.Unlock()

	w.removeTx(txID, err)
}

func (w *WebSocketServer) removeTx(txID ids.ID, err error) {
	listeners, ok := w.txWebSocketServer[txID]
	if !ok {
		return
	}
	bytes, _ := PackRemovedTxMessage(txID, err)
	w.s.Publish(bytes, listeners)
	delete(w.txWebSocketServer, txID)
	// [expiringTxs] will be cleared eventually (does not support removal)
}

func (w *WebSocketServer) SetMinTx(t int64) {
	w.txL.Lock()
	defer w.txL.Unlock()

	expired := w.expiringTxs.SetMin(t)
	for _, id := range expired {
		w.removeTx(id, ErrExpired)
	}
}

func (w *WebSocketServer) AcceptBlock(b *chain.StatelessBlock) {
	bytes, _ := PackBlockMessage(b)
	// TODO: only publish to block subscribers, can be a lot of extra bandwidth
	w.s.Publish(bytes, w.s.Connections())
	w.txL.Lock()
	defer w.txL.Unlock()
	results := b.Results()
	for i, tx := range b.Txs {
		txID := tx.ID()
		listeners, ok := w.txWebSocketServer[txID]
		if !ok {
			continue
		}
		// Publish to tx listener
		bytes, _ := PackAcceptedTxMessage(txID, results[i])
		w.s.Publish(
			bytes,
			listeners,
		)
		delete(w.txWebSocketServer, txID)
		// [expiringTxs] will be cleared eventually (does not support removal)
	}
}

func (w *WebSocketServer) MessageCallback(vm VM) pubsub.Callback {
	// Assumes controller is initialized before this is called
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

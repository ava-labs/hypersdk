// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/workers"
)

type txWrapper struct {
	msg []byte
	c   *pubsub.Connection
}

type WebSocketServer struct {
	vm VM
	s  *pubsub.Server

	blockListeners *pubsub.Connections
	chunkListeners *pubsub.Connections

	authWorkers          workers.Workers
	incomingTransactions chan *txWrapper
	txBacklog            atomic.Int64

	txL sync.Mutex
	// TODO: can unify with a single heap + wrapper
	txListeners map[ids.ID]*pubsub.Connections
	expiringTxs *emap.EMap[*chain.Transaction] // ensures all tx listeners are eventually responded to
}

func NewWebSocketServer(vm VM, maxPendingMessages int) (*WebSocketServer, *pubsub.Server) {
	w := &WebSocketServer{
		vm:                   vm,
		blockListeners:       pubsub.NewConnections(),
		chunkListeners:       pubsub.NewConnections(),
		incomingTransactions: make(chan *txWrapper, vm.GetAuthRPCBacklog()),
		txListeners:          map[ids.ID]*pubsub.Connections{},
		expiringTxs:          emap.NewEMap[*chain.Transaction](),
	}
	cfg := pubsub.NewDefaultServerConfig()
	cfg.MaxPendingMessages = maxPendingMessages
	w.s = pubsub.New(w.vm.Logger(), cfg, w.MessageCallback())
	for i := 0; i < vm.GetAuthRPCCores(); i++ {
		go w.startWorker()
	}
	return w, w.s
}

func (w *WebSocketServer) startWorker() {
	var (
		log                          = w.vm.Logger()
		actionRegistry, authRegistry = w.vm.Registry()
	)
	// We can't use batch verify here because we don't want to invalidate honest
	// submissions if one connection sent an invalid transaction.
	for {
		select {
		case txw := <-w.incomingTransactions:
			w.vm.RecordRPCTxBacklog(w.txBacklog.Add(-1))
			ctx := context.TODO()
			// Unmarshal TX
			p := codec.NewReader(txw.msg, consts.NetworkSizeLimit) // will likely be much smaller
			tx, err := chain.UnmarshalTx(p, actionRegistry, authRegistry)
			if err != nil {
				log.Error("failed to unmarshal tx",
					zap.Int("len", len(txw.msg)),
					zap.Error(err),
				)
				continue
			}

			// Verify tx
			if w.vm.GetVerifyAuth() {
				msg, err := tx.Digest()
				if err != nil {
					// Should never occur because populated during unmarshal
					continue
				}
				if err := tx.Auth.Verify(ctx, msg); err != nil {
					log.Error("failed to verify sig",
						zap.Error(err),
					)
					continue
				}
			}
			w.AddTxListener(tx, txw.c)

			// Submit will remove from [txWaiters] if it is not added
			txID := tx.ID()
			if err := w.vm.Submit(ctx, false, []*chain.Transaction{tx})[0]; err != nil {
				log.Error("failed to submit tx",
					zap.Stringer("txID", txID),
					zap.Error(err),
				)
				continue
			}

			// Prevent duplicate signature verification during block processing
			w.vm.AddRPCAuthorized(tx)
		case <-w.vm.StopChan():
			return
		}
	}
}

// Note: no need to have a tx listener removal, this will happen when all
// submitted transactions are cleared.
func (w *WebSocketServer) AddTxListener(tx *chain.Transaction, c *pubsub.Connection) {
	w.txL.Lock()
	defer w.txL.Unlock()

	// TODO: limit max number of tx listeners a single connection can create
	txID := tx.ID()
	if _, ok := w.txListeners[txID]; !ok {
		w.txListeners[txID] = pubsub.NewConnections()
	}
	w.txListeners[txID].Add(c)
	w.expiringTxs.Add([]*chain.Transaction{tx})
}

// If never possible for a tx to enter mempool, call this
func (w *WebSocketServer) RemoveTx(txID ids.ID, err error) error {
	w.txL.Lock()
	defer w.txL.Unlock()

	return w.removeTx(txID, err)
}

func (w *WebSocketServer) removeTx(txID ids.ID, err error) error {
	listeners, ok := w.txListeners[txID]
	if !ok {
		return nil
	}
	bytes, err := PackRemovedTxMessage(txID, err)
	if err != nil {
		return err
	}
	w.s.Publish(append([]byte{TxMode}, bytes...), listeners)
	delete(w.txListeners, txID)
	// [expiringTxs] will be cleared eventually (does not support removal)
	return nil
}

func (w *WebSocketServer) SetMinTx(t int64) error {
	w.txL.Lock()
	defer w.txL.Unlock()

	expired := w.expiringTxs.SetMin(t)
	for _, id := range expired {
		if err := w.removeTx(id, ErrExpired); err != nil {
			return err
		}
	}
	if exp := len(expired); exp > 0 {
		w.vm.Logger().Debug("expired listeners", zap.Int("count", exp))
	}
	return nil
}

func (w *WebSocketServer) AcceptBlock(b *chain.StatelessBlock) error {
	if w.blockListeners.Len() > 0 {
		inactiveConnection := w.s.Publish(append([]byte{BlockMode}, b.Bytes()...), w.blockListeners)
		for _, conn := range inactiveConnection {
			w.blockListeners.Remove(conn)
		}
	}
	return nil
}

func (w *WebSocketServer) ExecuteChunk(blk uint64, chunk *chain.FilteredChunk, results []*chain.Result) error {
	if w.chunkListeners.Len() > 0 {
		bytes, err := PackChunkMessage(blk, chunk, results)
		if err != nil {
			return err
		}
		inactiveConnection := w.s.Publish(append([]byte{ChunkMode}, bytes...), w.chunkListeners)
		for _, conn := range inactiveConnection {
			w.chunkListeners.Remove(conn)
		}
	}

	w.txL.Lock()
	defer w.txL.Unlock()
	for i, tx := range chunk.Txs {
		txID := tx.ID()
		listeners, ok := w.txListeners[txID]
		if !ok {
			continue
		}
		// Publish to tx listener
		bytes, err := PackAcceptedTxMessage(txID, results[i])
		if err != nil {
			return err
		}
		w.s.Publish(append([]byte{TxMode}, bytes...), listeners)
		delete(w.txListeners, txID)
		// [expiringTxs] will be cleared eventually (does not support removal)
	}
	return nil
}

func (w *WebSocketServer) MessageCallback() pubsub.Callback {
	// Assumes controller is initialized before this is called
	var (
		tracer = w.vm.Tracer()
		log    = w.vm.Logger()
	)

	return func(msgBytes []byte, c *pubsub.Connection) {
		_, span := tracer.Start(context.Background(), "WebSocketServer.Callback")
		defer span.End()

		// Check empty messages
		if len(msgBytes) == 0 {
			log.Error("failed to unmarshal msg",
				zap.Int("len", len(msgBytes)),
			)
			return
		}

		// TODO: convert into a router that can be re-used in custom WS
		// implementations
		switch msgBytes[0] {
		case BlockMode:
			w.blockListeners.Add(c)
			log.Debug("added block listener")
		case ChunkMode:
			w.chunkListeners.Add(c)
			log.Debug("added chunk listener")
		case TxMode:
			msgBytes = msgBytes[1:]
			select {
			case w.incomingTransactions <- &txWrapper{msgBytes, c}:
				w.txBacklog.Add(1)
				log.Debug("enqueued tx for processing")
			default:
				log.Debug("dropping tx because backlog is full")
			}
		default:
			log.Error("unexpected message type",
				zap.Int("len", len(msgBytes)),
				zap.Uint8("mode", msgBytes[0]),
			)
		}
	}
}

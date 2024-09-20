// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ws

import (
	"context"
	"errors"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/emap"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/vm"
)

const (
	Endpoint  = "/corews"
	Namespace = "websocket"
)

var (
	_ api.HandlerFactory[api.VM[chain.RuntimeInterface]] = (*WebSocketServerFactory[chain.RuntimeInterface])(nil)

	ErrExpired = errors.New("expired")
)

type Config struct {
	Enabled            bool `json:"enabled"`
	MaxPendingMessages int  `json:"maxPendingMessages"`
}

func NewDefaultConfig() Config {
	return Config{
		Enabled:            true,
		MaxPendingMessages: 10_000_000,
	}
}

func With[T chain.RuntimeInterface]() vm.Option[T] {
	return vm.NewOption[T](Namespace, NewDefaultConfig(), OptionFunc[T])
}

func OptionFunc[T chain.RuntimeInterface](v *vm.VM[T], config Config) error {
	if !config.Enabled {
		return nil
	}

	actionRegistry, authRegistry := v.ActionRegistry(), v.AuthRegistry()
	server, handler := NewWebSocketServer[T](
		v,
		v.Logger(),
		v.Tracer(),
		actionRegistry,
		authRegistry,
		config.MaxPendingMessages,
	)

	webSocketFactory := NewWebSocketServerFactory[T](handler)
	txRemovedSubscription := event.SubscriptionFuncFactory[vm.TxRemovedEvent]{
		AcceptF: func(event vm.TxRemovedEvent) error {
			return server.RemoveTx(event.TxID, event.Err)
		},
	}

	blockSubscription := event.SubscriptionFuncFactory[*chain.StatefulBlock[T]]{
		AcceptF: func(event *chain.StatefulBlock[T]) error {
			return server.AcceptBlock(event)
		},
	}

	vm.WithBlockSubscriptions[T](blockSubscription)(v)
	vm.WithTxRemovedSubscriptions[T](txRemovedSubscription)(v)
	vm.WithVMAPIs[T](webSocketFactory)(v)

	return nil
}

func NewWebSocketServerFactory[T chain.RuntimeInterface](server *pubsub.Server) *WebSocketServerFactory[T] {
	return &WebSocketServerFactory[T]{
		handler: server,
	}
}

type WebSocketServerFactory[T chain.RuntimeInterface] struct {
	handler *pubsub.Server
}

func (w WebSocketServerFactory[T]) New(api.VM[T]) (api.Handler, error) {
	return api.Handler{
		Path:    Endpoint,
		Handler: w.handler,
	}, nil
}

type WebSocketServer[T chain.RuntimeInterface] struct {
	vm             api.VM[T]
	logger         logging.Logger
	tracer         trace.Tracer
	actionRegistry chain.ActionRegistry[T]
	authRegistry   chain.AuthRegistry

	s *pubsub.Server

	blockListeners *pubsub.Connections

	txL         sync.Mutex
	txListeners map[ids.ID]*pubsub.Connections
	expiringTxs *emap.EMap[*chain.Transaction[T]] // ensures all tx listeners are eventually responded to
}

func NewWebSocketServer[T chain.RuntimeInterface](
	vm api.VM[T],
	log logging.Logger,
	tracer trace.Tracer,
	actionRegistry chain.ActionRegistry[T],
	authRegistry chain.AuthRegistry,
	maxPendingMessages int,
) (*WebSocketServer[T], *pubsub.Server) {
	w := &WebSocketServer[T]{
		vm:             vm,
		logger:         log,
		tracer:         tracer,
		actionRegistry: actionRegistry,
		authRegistry:   authRegistry,
		blockListeners: pubsub.NewConnections(),
		txListeners:    map[ids.ID]*pubsub.Connections{},
		expiringTxs:    emap.NewEMap[*chain.Transaction[T]](),
	}
	cfg := pubsub.NewDefaultServerConfig()
	cfg.MaxPendingMessages = maxPendingMessages
	w.s = pubsub.New(w.logger, cfg, w.MessageCallback())
	return w, w.s
}

// Note: no need to have a tx listener removal, this will happen when all
// submitted transactions are cleared.
func (w *WebSocketServer[T]) AddTxListener(tx *chain.Transaction[T], c *pubsub.Connection) {
	w.txL.Lock()
	defer w.txL.Unlock()

	// TODO: limit max number of tx listeners a single connection can create
	txID := tx.ID()
	if _, ok := w.txListeners[txID]; !ok {
		w.txListeners[txID] = pubsub.NewConnections()
	}
	w.txListeners[txID].Add(c)
	w.expiringTxs.Add([]*chain.Transaction[T]{tx})
}

// If never possible for a tx to enter mempool, call this
func (w *WebSocketServer[T]) RemoveTx(txID ids.ID, err error) error {
	w.txL.Lock()
	defer w.txL.Unlock()

	return w.removeTx(txID, err)
}

func (w *WebSocketServer[T]) removeTx(txID ids.ID, err error) error {
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

func (w *WebSocketServer[T]) setMinTx(t int64) error {
	expired := w.expiringTxs.SetMin(t)
	for _, id := range expired {
		if err := w.removeTx(id, ErrExpired); err != nil {
			return err
		}
	}
	if exp := len(expired); exp > 0 {
		w.logger.Debug("expired listeners", zap.Int("count", exp))
	}
	return nil
}

func (w *WebSocketServer[T]) AcceptBlock(b *chain.StatefulBlock[T]) error {
	if w.blockListeners.Len() > 0 {
		bytes, err := PackBlockMessage[T](b)
		if err != nil {
			return err
		}
		inactiveConnection := w.s.Publish(append([]byte{BlockMode}, bytes...), w.blockListeners)
		for _, conn := range inactiveConnection {
			w.blockListeners.Remove(conn)
		}
	}

	w.txL.Lock()
	defer w.txL.Unlock()
	results := b.Results()
	for i, tx := range b.Txs {
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
	return w.setMinTx(b.Tmstmp)
}

func (w *WebSocketServer[T]) MessageCallback() pubsub.Callback {
	return func(msgBytes []byte, c *pubsub.Connection) {
		ctx, span := w.tracer.Start(context.Background(), "WebSocketServer.Callback")
		defer span.End()

		// Check empty messages
		if len(msgBytes) == 0 {
			w.logger.Error("failed to unmarshal msg",
				zap.Int("len", len(msgBytes)),
			)
			return
		}

		// TODO: convert into a router that can be re-used in custom WS
		// implementations
		switch msgBytes[0] {
		case BlockMode:
			w.blockListeners.Add(c)
			w.logger.Debug("added block listener")
		case TxMode:
			msgBytes = msgBytes[1:]
			// Unmarshal TX
			p := codec.NewReader(msgBytes, consts.NetworkSizeLimit) // will likely be much smaller
			tx, err := chain.UnmarshalTx[T](p, w.actionRegistry, w.authRegistry)
			if err != nil {
				w.logger.Error("failed to unmarshal tx",
					zap.Int("len", len(msgBytes)),
					zap.Error(err),
				)
				return
			}

			// Verify tx
			if w.vm.GetVerifyAuth() {
				msg, err := tx.Digest()
				if err != nil {
					// Should never occur because populated during unmarshal
					return
				}
				if err := tx.Auth.Verify(ctx, msg); err != nil {
					w.logger.Error("failed to verify sig",
						zap.Error(err),
					)
					return
				}
			}
			w.AddTxListener(tx, c)

			// Submit will remove from [txWaiters] if it is not added
			txID := tx.ID()
			if err := w.vm.Submit(ctx, false, []*chain.Transaction[T]{tx})[0]; err != nil {
				w.logger.Error("failed to submit tx",
					zap.Stringer("txID", txID),
					zap.Error(err),
				)
				return
			}
			w.logger.Debug("submitted tx", zap.Stringer("id", txID))
		default:
			w.logger.Error("unexpected message type",
				zap.Int("len", len(msgBytes)),
				zap.Uint8("mode", msgBytes[0]),
			)
		}
	}
}

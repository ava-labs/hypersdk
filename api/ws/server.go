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
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/vm"
)

const Endpoint = "/corews"

var (
	_ api.HandlerFactory[api.VM]          = (*WebSocketServerFactory)(nil)
	_ event.Subscription[struct{}]        = (*subscriptionFunc[struct{}])(nil)
	_ event.SubscriptionFactory[struct{}] = (*subscriptionFuncFactory[struct{}])(nil)

	ErrExpired = errors.New("expired")
)

func WithWebsocketAPI(
	log logging.Logger,
	tracer trace.Tracer,
	actionRegistry *codec.TypeParser[chain.Action],
	authRegistry *codec.TypeParser[chain.Auth],
	maxPendingMessages int,
) vm.Option {
	return func(v *vm.VM) error {
		server, handler := NewWebSocketServer(
			v,
			log,
			tracer,
			actionRegistry,
			authRegistry,
			maxPendingMessages,
		)

		webSocketFactory := NewWebSocketServerFactory(handler)
		txRemovedSubscription := subscriptionFuncFactory[vm.TxRemovedEvent]{
			AcceptF: func(event vm.TxRemovedEvent) error {
				return server.RemoveTx(event.TxID, event.Err)
			},
		}

		blockSubscription := subscriptionFuncFactory[*chain.StatelessBlock]{
			AcceptF: func(event *chain.StatelessBlock) error {
				return server.AcceptBlock(event)
			},
		}

		if err := vm.WithBlockSubscriptions(blockSubscription)(v); err != nil {
			return err
		}

		if err := vm.WithTxRemovedSubscriptions(txRemovedSubscription)(v); err != nil {
			return err
		}

		return vm.WithVMAPIs(webSocketFactory)(v)
	}
}

type subscriptionFuncFactory[T any] struct {
	AcceptF func(t T) error
}

func (s subscriptionFuncFactory[T]) New() (event.Subscription[T], error) {
	return subscriptionFunc[T](s), nil
}

type subscriptionFunc[T any] struct {
	AcceptF func(t T) error
}

func (s subscriptionFunc[T]) Accept(t T) error {
	return s.AcceptF(t)
}

func (subscriptionFunc[_]) Close() error {
	return nil
}

func NewWebSocketServerFactory(server *pubsub.Server) *WebSocketServerFactory {
	return &WebSocketServerFactory{
		handler: server,
	}
}

type WebSocketServerFactory struct {
	handler *pubsub.Server
}

func (w WebSocketServerFactory) New(api.VM) (api.Handler, error) {
	return api.Handler{
		Path:    Endpoint,
		Handler: w.handler,
	}, nil
}

type WebSocketServer struct {
	vm             api.VM
	logger         logging.Logger
	tracer         trace.Tracer
	actionRegistry chain.ActionRegistry
	authRegistry   chain.AuthRegistry

	s *pubsub.Server

	blockListeners *pubsub.Connections

	txL         sync.Mutex
	txListeners map[ids.ID]*pubsub.Connections
	expiringTxs *emap.EMap[*chain.Transaction] // ensures all tx listeners are eventually responded to
}

func NewWebSocketServer(
	vm api.VM,
	log logging.Logger,
	tracer trace.Tracer,
	actionRegistry chain.ActionRegistry,
	authRegistry chain.AuthRegistry,
	maxPendingMessages int,
) (*WebSocketServer, *pubsub.Server) {
	w := &WebSocketServer{
		vm:             vm,
		logger:         log,
		tracer:         tracer,
		actionRegistry: actionRegistry,
		authRegistry:   authRegistry,
		blockListeners: pubsub.NewConnections(),
		txListeners:    map[ids.ID]*pubsub.Connections{},
		expiringTxs:    emap.NewEMap[*chain.Transaction](),
	}
	cfg := pubsub.NewDefaultServerConfig()
	cfg.MaxPendingMessages = maxPendingMessages
	w.s = pubsub.New(w.logger, cfg, w.MessageCallback())
	return w, w.s
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

func (w *WebSocketServer) setMinTx(t int64) error {
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

func (w *WebSocketServer) AcceptBlock(b *chain.StatelessBlock) error {
	if w.blockListeners.Len() > 0 {
		bytes, err := PackBlockMessage(b)
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

func (w *WebSocketServer) MessageCallback() pubsub.Callback {
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
			tx, err := chain.UnmarshalTx(p, w.actionRegistry, w.authRegistry)
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
			if err := w.vm.Submit(ctx, false, []*chain.Transaction{tx})[0]; err != nil {
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

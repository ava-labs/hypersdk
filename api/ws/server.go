// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ws

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/emap"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/vm"
)

const (
	Endpoint  = "/corews"
	Namespace = "websocket"
)

var _ api.HandlerFactory[api.VM] = (*WebSocketServerFactory)(nil)

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

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), OptionFunc)
}

func OptionFunc(v api.VM, config Config) (vm.Opt, error) {
	if !config.Enabled {
		return vm.NewOpt(), nil
	}

	server, handler := NewWebSocketServer(
		v,
		v.Logger(),
		v.Tracer(),
		v.GetParser(),
		config.MaxPendingMessages,
	)

	webSocketFactory := NewWebSocketServerFactory(handler)

	blockSubscription := event.SubscriptionFuncFactory[*chain.ExecutedBlock]{
		NotifyF: func(ctx context.Context, event *chain.ExecutedBlock) error {
			return server.AcceptBlock(ctx, event)
		},
	}

	return vm.NewOpt(
		vm.WithBlockSubscriptions(blockSubscription),
		vm.WithVMAPIs(webSocketFactory),
	), nil
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
	vm     api.VM
	logger logging.Logger
	tracer trace.Tracer
	parser chain.Parser

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
	txParser chain.Parser,
	maxPendingMessages int,
) (*WebSocketServer, *pubsub.Server) {
	w := &WebSocketServer{
		vm:             vm,
		logger:         log,
		tracer:         tracer,
		parser:         txParser,
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
	txID := tx.GetID()
	connections, ok := w.txListeners[txID]
	if !ok {
		connections = pubsub.NewConnections()
		w.txListeners[txID] = connections
	}
	connections.Add(c)
	w.expiringTxs.Add([]*chain.Transaction{tx})
}

func (w *WebSocketServer) expireTx(txID ids.ID) {
	listeners, ok := w.txListeners[txID]
	if !ok {
		return
	}
	// nil result indicates the transaction expired
	bytes := packTxMessage(txID, nil)
	w.s.Publish(append([]byte{TxMode}, bytes...), listeners)
	delete(w.txListeners, txID)
	// [expiringTxs] will be cleared eventually (does not support removal)
}

func (w *WebSocketServer) setMinTx(t int64) {
	expired := w.expiringTxs.SetMin(t)
	for _, id := range expired {
		w.expireTx(id)
	}
	if exp := len(expired); exp > 0 {
		w.logger.Debug("expired listeners", zap.Int("count", exp))
	}
}

func (w *WebSocketServer) AcceptBlock(_ context.Context, b *chain.ExecutedBlock) error {
	if w.blockListeners.Len() > 0 {
		bytes, err := b.Marshal()
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
	results := b.ExecutionResults.Results
	for i, tx := range b.Block.Txs {
		txID := tx.GetID()
		listeners, ok := w.txListeners[txID]
		if !ok {
			continue
		}
		// Publish to tx listener
		bytes := packTxMessage(txID, results[i])

		// Skip clearing inactive connections because they'll be deleted
		// regardless.
		_ = w.s.Publish(append([]byte{TxMode}, bytes...), listeners)
		delete(w.txListeners, txID)
		// [expiringTxs] will be cleared eventually (does not support removal)
	}
	w.setMinTx(b.Block.Tmstmp)
	return nil
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
			tx, err := chain.UnmarshalTx(msgBytes, w.parser)
			if err != nil {
				w.logger.Error("failed to unmarshal tx",
					zap.Int("len", len(msgBytes)),
					zap.Error(err),
				)
				return
			}

			w.AddTxListener(tx, c)

			// Submit will remove from [txListeners] if it is not added
			txID := tx.GetID()
			if err := w.vm.Submit(ctx, []*chain.Transaction{tx})[0]; err != nil {
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

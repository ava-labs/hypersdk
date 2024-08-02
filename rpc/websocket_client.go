// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/gorilla/websocket"
)

type byteWrapper struct {
	t int64
	b []byte
}

// TODO: move client connection logic to pubsub (so much shared with connection, it is a bit silly)
type WebSocketClient struct {
	cl   sync.Once
	conn *websocket.Conn

	mb           *pubsub.MessageBuffer
	writeStopped chan struct{}
	readStopped  chan struct{}

	preConfsMap     map[ids.ID]bool
	pendingBlocks   chan []byte
	pendingChunks   chan []byte
	pendingPreConfs chan []byte
	pendingTxs      chan *byteWrapper
	txsToProcess    atomic.Int64

	txsl sync.Mutex
	txs  map[uint64]ids.ID

	startedClose bool
	closed       bool
	err          error
	errl         sync.Once
}

// NewWebSocketClient creates a new client for the decision rpc server.
// Dials into the server at [uri] and returns a client.
func NewWebSocketClient(uri string, handshakeTimeout time.Duration, pending, targetWrite, maxWrite int) (*WebSocketClient, error) {
	uri = strings.ReplaceAll(uri, "http://", "ws://")
	uri = strings.ReplaceAll(uri, "https://", "wss://")
	if !strings.HasPrefix(uri, "ws") { // fallback to default usage
		uri = "ws://" + uri
	}
	uri = strings.TrimSuffix(uri, "/")
	uri += WebSocketEndpoint
	// source: https://github.com/gorilla/websocket/blob/76ecc29eff79f0cedf70c530605e486fc32131d1/client.go#L140-L144
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: handshakeTimeout,
		ReadBufferSize:   pubsub.MaxWriteMessageSize, // we assume we are reading from server (TODO: make generic)
		WriteBufferSize:  maxWrite,
	}
	conn, resp, err := dialer.Dial(uri, nil)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	wc := &WebSocketClient{
		conn:            conn,
		mb:              pubsub.NewMessageBuffer(&logging.NoLog{}, pending, targetWrite, maxWrite, pubsub.MaxMessageWait),
		readStopped:     make(chan struct{}),
		writeStopped:    make(chan struct{}),
		preConfsMap:     make(map[ids.ID]bool),
		pendingBlocks:   make(chan []byte, pending),
		pendingChunks:   make(chan []byte, pending),
		pendingPreConfs: make(chan []byte, pending),
		pendingTxs:      make(chan *byteWrapper, pending),
		txs:             make(map[uint64]ids.ID, pending),
	}
	go func() {
		defer close(wc.readStopped)

		// Ensure connection stays open as long as we get pings
		//
		// Note: the server is doing the same thing to prune connections...we
		// should unify this logic in the future.
		if err := wc.conn.SetReadDeadline(time.Now().Add(pubsub.PongWait)); err != nil {
			return
		}
		wc.conn.SetPongHandler(func(string) error {
			return wc.conn.SetReadDeadline(time.Now().Add(pubsub.PongWait))
		})

		// TODO: add ReadLimit
		for {
			messageType, msgBatch, err := conn.ReadMessage()
			if err != nil {
				wc.errl.Do(func() {
					wc.err = err
				})
				return
			}
			if messageType != websocket.BinaryMessage {
				continue
			}
			if len(msgBatch) == 0 {
				utils.Outf("{{orange}}got empty message{{/}}\n")
				continue
			}
			msgs, err := pubsub.ParseBatchMessage(pubsub.MaxWriteMessageSize, msgBatch)
			if err != nil {
				utils.Outf("{{orange}}received invalid message:{{/}} %v\n", err)
				continue
			}
			for _, msg := range msgs {
				tmsg := msg[1:]
				switch msg[0] {
				case BlockMode:
					wc.pendingBlocks <- tmsg
				case ChunkMode:
					wc.pendingChunks <- tmsg
				case TxMode:
					wc.txsToProcess.Add(1)
					wc.pendingTxs <- &byteWrapper{t: time.Now().UnixMilli(), b: tmsg}
				case PreConfMode:
					wc.pendingPreConfs <- tmsg
				default:
					utils.Outf("{{orange}}unexpected message mode:{{/}} %x\n", msg[0])
					continue
				}
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(pubsub.PingPeriod)
		defer func() {
			ticker.Stop()
			close(wc.writeStopped)
		}()
		for {
			select {
			case msg, ok := <-wc.mb.Queue:
				if !ok {
					return
				}
				wc.conn.SetWriteDeadline(time.Now().Add(pubsub.WriteWait))
				if err := wc.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
					wc.errl.Do(func() {
						wc.err = err
					})
					_ = wc.conn.Close()
					return
				}
			case <-ticker.C:
				wc.conn.SetWriteDeadline(time.Now().Add(pubsub.WriteWait))
				if err := wc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					wc.errl.Do(func() {
						wc.err = err
					})
					_ = wc.conn.Close()
					return
				}
			case <-wc.readStopped:
				// If we exit here, the connection must've failed ungracefully
				// otherwise writeStopped will exit first.
				_ = wc.mb.Close()
				return
			}
		}
	}()
	go func() {
		<-wc.writeStopped
		<-wc.readStopped
		if !wc.startedClose {
			utils.Outf("{{orange}}unclean client shutdown:{{/}} %v\n", wc.err)
		}
		wc.closed = true
	}()
	return wc, nil
}

func (c *WebSocketClient) RegisterBlocks() error {
	if c.closed {
		return ErrClosed
	}
	_, err := c.mb.Send([]byte{BlockMode})
	return err
}

// Listen listens for block messages from the streaming server.
func (c *WebSocketClient) ListenBlock(
	ctx context.Context,
) (*chain.StatefulBlock, []ids.ID, error) {
	select {
	case msg := <-c.pendingBlocks:
		// @todo match the preconfs from the cached values
		blk, err := chain.UnmarshalBlock(msg)
		if err != nil {
			return nil, nil, err
		}
		avalChunkIDs := make([]ids.ID, 0)
		for _, chunk := range blk.AvailableChunks {
			avalChunkIDs = append(avalChunkIDs, chunk.ID())
		}
		return blk, avalChunkIDs, nil
	case <-c.readStopped:
		return nil, nil, c.err
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

func (c *WebSocketClient) RegisterChunks() error {
	if c.closed {
		return ErrClosed
	}
	_, err := c.mb.Send([]byte{ChunkMode})
	return err
}

// Listen listens for block messages from the streaming server.
func (c *WebSocketClient) ListenChunk(
	ctx context.Context,
	parser chain.Parser,
) (uint64, *chain.FilteredChunk, []*chain.Result, error) {
	select {
	case msg := <-c.pendingChunks:
		return UnpackChunkMessage(msg, parser)
	case <-c.readStopped:
		return 0, nil, nil, c.err
	case <-ctx.Done():
		return 0, nil, nil, ctx.Err()
	}
}

func (c *WebSocketClient) RegisterPreConf() error {
	if c.closed {
		return ErrClosed
	}
	_, err := c.mb.Send([]byte{PreConfMode})
	return err
}

func (c *WebSocketClient) ListenPreConf(ctx context.Context) (ids.ID, error) {
	select {
	case msg := <-c.pendingPreConfs:
		return ids.FromString(string(msg))
	case <-c.readStopped:
		return ids.Empty, c.err
	case <-ctx.Done():
		return ids.Empty, ctx.Err()
	}
}

// IssueTx sends [tx] to the streaming rpc server.
func (c *WebSocketClient) RegisterTx(tx *chain.Transaction) error {
	if c.closed {
		return ErrClosed
	}
	txC, err := c.mb.Send(append([]byte{TxMode}, tx.Bytes()...))
	if err != nil {
		return err
	}
	txID := tx.ID()
	c.txsl.Lock()
	c.txs[txC] = txID
	c.txsl.Unlock()
	return nil
}

// ListenForTx listens for responses from the streamingServer.
//
// TODO: add the option to subscribe to a single TxID to avoid
// trampling other listeners (could have an intermediate tracking
// layer in the client so no changes required in the server).
func (c *WebSocketClient) ListenTx(ctx context.Context) (int64, int, ids.ID, uint8, error) {
	select {
	case bw := <-c.pendingTxs:
		c.txsToProcess.Add(-1)
		rtxID, status, err := UnpackTxMessage(bw.b)
		if err != nil {
			return -1, 0, ids.Empty, 0, err
		}
		c.txsl.Lock()
		defer c.txsl.Unlock()
		txID, ok := c.txs[rtxID]
		if !ok {
			return -1, 0, ids.Empty, 0, ErrUnknownTx
		}
		delete(c.txs, rtxID)
		return bw.t, len(bw.b), txID, status, nil
	case <-c.readStopped:
		return -1, 0, ids.Empty, 0, c.err
	case <-ctx.Done():
		return -1, 0, ids.Empty, 0, ctx.Err()
	}
}

// Close closes [c]'s connection to the decision rpc server.
func (c *WebSocketClient) Close() error {
	// Check if we already exited
	select {
	case <-c.readStopped:
		return c.err
	default:
	}

	var err error
	c.cl.Do(func() {
		c.startedClose = true

		// Flush all unwritten messages before we close the connection
		_ = c.mb.Close()
		<-c.writeStopped

		// Close connection and stop reading
		_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
		err = c.conn.Close()
	})
	return err
}

func (c *WebSocketClient) TxsToProcess() int64 {
	return c.txsToProcess.Load()
}

func (c *WebSocketClient) Closed() bool {
	return c.closed
}

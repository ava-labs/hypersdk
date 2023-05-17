// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"strings"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	cl   sync.Once
	conn *websocket.Conn

	mb           *pubsub.MessageBuffer
	writeStopped chan struct{}
	readStopped  chan struct{}

	pendingBlocks chan []byte
	pendingTxs    chan []byte

	err error
}

// NewWebSocketClient creates a new client for the decision rpc server.
// Dials into the server at [uri] and returns a client.
func NewWebSocketClient(uri string, pending int, maxSize int) (*WebSocketClient, error) {
	uri = strings.ReplaceAll(uri, "http://", "ws://")
	uri = strings.ReplaceAll(uri, "https://", "wss://")
	if !strings.HasPrefix(uri, "ws") { // fallback to default usage
		uri = "ws://" + uri
	}
	uri = strings.TrimSuffix(uri, "/")
	uri += WebSocketEndpoint
	conn, resp, err := websocket.DefaultDialer.Dial(uri, nil)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	wc := &WebSocketClient{
		conn:          conn,
		mb:            pubsub.NewMessageBuffer(&logging.NoLog{}, pending, maxSize, pubsub.MaxMessageWait),
		readStopped:   make(chan struct{}),
		writeStopped:  make(chan struct{}),
		pendingBlocks: make(chan []byte, pending),
		pendingTxs:    make(chan []byte, pending),
	}
	go func() {
		defer close(wc.readStopped)
		for {
			_, msgBatch, err := conn.ReadMessage()
			if err != nil {
				wc.err = err
				return
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
				case TxMode:
					wc.pendingTxs <- tmsg
				default:
					utils.Outf("{{orange}}unexpected message mode:{{/}} %x\n", msg[0])
					continue
				}
			}
		}
	}()
	go func() {
		defer close(wc.writeStopped)
		for {
			select {
			case msg, ok := <-wc.mb.Queue:
				if !ok {
					return
				}
				if err := wc.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
					utils.Outf("{{orange}}unable to write message:{{/}} %v\n", err)
				}
			case <-wc.readStopped:
				// If we exit here, the connection must've failed ungracefully
				// otherwise writeStopped will exit first.
				_ = wc.mb.Close()
				return
			}
		}
	}()
	return wc, nil
}

func (c *WebSocketClient) RegisterBlocks() error {
	return c.mb.Send([]byte{BlockMode})
}

// Listen listens for block messages from the streaming server.
func (c *WebSocketClient) ListenBlock(
	ctx context.Context,
	parser chain.Parser,
) (*chain.RootBlock, []*chain.TxBlock, []*chain.Result, error) {
	select {
	case msg := <-c.pendingBlocks:
		return UnpackBlockMessage(msg, parser)
	case <-c.readStopped:
		return nil, nil, nil, c.err
	case <-ctx.Done():
		return nil, nil, nil, ctx.Err()
	}
}

// IssueTx sends [tx] to the streaming rpc server.
func (c *WebSocketClient) RegisterTx(tx *chain.Transaction) error {
	return c.mb.Send(append([]byte{TxMode}, tx.Bytes()...))
}

// ListenForTx listens for responses from the streamingServer.
func (c *WebSocketClient) ListenTx(ctx context.Context) (ids.ID, error, *chain.Result, error) {
	select {
	case msg := <-c.pendingTxs:
		return UnpackTxMessage(msg)
	case <-c.readStopped:
		return ids.Empty, nil, nil, c.err
	case <-ctx.Done():
		return ids.Empty, nil, nil, ctx.Err()
	}
}

// Close closes [c]'s connection to the decision rpc server.
func (c *WebSocketClient) Close() error {
	var err error
	c.cl.Do(func() {
		// Flush all unwritten messages before we close the connection
		_ = c.mb.Close()
		<-c.writeStopped

		// Close connection and stop reading
		err = c.conn.Close()
	})
	return err
}

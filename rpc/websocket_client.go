// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"strings"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/utils"
	"github.com/gorilla/websocket"
)

const pendingChanSize = 8_192

type WebSocketClient struct {
	conn *websocket.Conn
	wl   sync.Mutex
	cl   sync.Once

	pendingBlocks chan []byte
	pendingTxs    chan []byte

	done chan struct{}
	err  error
}

// NewWebSocketClient creates a new client for the decision rpc server.
// Dials into the server at [uri] and returns a client.
func NewWebSocketClient(uri string) (*WebSocketClient, error) {
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
		pendingBlocks: make(chan []byte, pendingChanSize),
		pendingTxs:    make(chan []byte, pendingChanSize),
		done:          make(chan struct{}),
	}
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				wc.err = err
				close(wc.done)
				return
			}
			if len(msg) == 0 {
				utils.Outf("{{orange}}got empty message{{/}}\n")
				continue
			}
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
	}()
	return wc, nil
}

func (c *WebSocketClient) RegisterBlocks() error {
	c.wl.Lock()
	defer c.wl.Unlock()

	return c.conn.WriteMessage(websocket.BinaryMessage, []byte{BlockMode})
}

// Listen listens for block messages from the streaming server.
func (c *WebSocketClient) ListenBlock(
	ctx context.Context,
	parser chain.Parser,
) (*chain.StatefulBlock, []*chain.Result, error) {
	select {
	case msg := <-c.pendingBlocks:
		return UnpackBlockMessage(msg, parser)
	case <-c.done:
		return nil, nil, c.err
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// IssueTx sends [tx] to the streaming rpc server.
func (c *WebSocketClient) RegisterTx(tx *chain.Transaction) error {
	c.wl.Lock()
	defer c.wl.Unlock()

	return c.conn.WriteMessage(websocket.BinaryMessage, append([]byte{TxMode}, tx.Bytes()...))
}

// ListenForTx listens for responses from the streamingServer.
func (c *WebSocketClient) ListenTx(ctx context.Context) (ids.ID, error, *chain.Result, error) {
	select {
	case msg := <-c.pendingTxs:
		return UnpackTxMessage(msg)
	case <-c.done:
		return ids.Empty, nil, nil, c.err
	case <-ctx.Done():
		return ids.Empty, nil, nil, ctx.Err()
	}
}

// Close closes [c]'s connection to the decision rpc server.
func (c *WebSocketClient) Close() error {
	var err error
	c.cl.Do(func() {
		err = c.conn.Close()
	})
	return err
}

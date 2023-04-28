package rpc

import (
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	conn *websocket.Conn
	wl   sync.Mutex
	dll  sync.Mutex
	bll  sync.Mutex
	cl   sync.Once
}

// NewWebSocketClient creates a new client for the decision rpc server.
// Dials into the server at [uri] and returns a client.
func NewWebSocketClient(uri string) (*WebSocketClient, error) {
	conn, resp, err := websocket.DefaultDialer.Dial(uri, nil)
	if err != nil {
		return nil, err
	}
	// not using resp for now
	resp.Body.Close()
	return &WebSocketClient{conn: conn}, nil
}

// IssueTx sends [tx] to the streaming rpc server.
func (c *WebSocketClient) IssueTx(tx *chain.Transaction) error {
	c.wl.Lock()
	defer c.wl.Unlock()

	return c.conn.WriteMessage(websocket.BinaryMessage, tx.Bytes())
}

// ListenForTx listens for responses from the streamingServer.
func (c *WebSocketClient) ListenForTx() (ids.ID, error, *chain.Result, error) {
	c.dll.Lock()
	defer c.dll.Unlock()
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return ids.Empty, nil, nil, err
		}
		if msg == nil || msg[0] == DecisionMode {
			return UnpackTxMessage(msg)
		}
	}
}

// Listen listens for block messages from the streaming server.
func (c *WebSocketClient) ListenForBlock(
	parser chain.Parser,
) (*chain.StatefulBlock, []*chain.Result, error) {
	// TODO: remove locks
	c.bll.Lock()
	defer c.bll.Unlock()

	for {
		// TODO: need to have a single router or will read from shared conn
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return nil, nil, err
		}
		if msg == nil || msg[0] == BlockMode {
			return UnpackBlockMessageBytes(msg, parser)
		}
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

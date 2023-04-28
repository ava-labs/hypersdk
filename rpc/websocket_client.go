package rpc

import (
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	conn *websocket.Conn
	cl   sync.Once

	// TODO: don't enqueue if not listening?
	pending
	err error
}

// NewWebSocketClient creates a new client for the decision rpc server.
// Dials into the server at [uri] and returns a client.
func NewWebSocketClient(uri string) (*WebSocketClient, error) {
	conn, resp, err := websocket.DefaultDialer.Dial(uri, nil)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	wc := &WebSocketClient{conn: conn}
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				wc.err = err
				return
			}
			switch msg[0] {
			case DecisionMode:
			case BlockMode:
			default:
				utils.Outf("{[orange}}unexpected message mode:{{/}} %x\n", msg[0])
				continue
			}
		}
	}()
	return wc, nil
}

// IssueTx sends [tx] to the streaming rpc server.
func (c *WebSocketClient) IssueTx(tx *chain.Transaction) error {
	return c.conn.WriteMessage(websocket.BinaryMessage, tx.Bytes())
}

// ListenForTx listens for responses from the streamingServer.
func (c *WebSocketClient) ListenForTx() (ids.ID, error, *chain.Result, error) {
	// TODO: need to send listen for block?
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

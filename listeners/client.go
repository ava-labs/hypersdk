package listeners

import (
	"net/url"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/gorilla/websocket"
)

// If you don't keep up, you will data
type Client struct {
	conn *websocket.Conn
	wl   sync.Mutex
	dll  sync.Mutex
	bll  sync.Mutex
	cl   sync.Once
}

// NewDecisionRPCClient creates a new client for the decision rpc server.
// Dials into the server at [uri] and returns a client.
func NewStreamingClient(uri string) (*Client, error) {
	// nil for now until we want to pass in headers
	u := url.URL{Scheme: "ws", Host: uri}
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	// not using resp for now
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

// IssueTx sends [tx] to the streaming rpc server.
func (c *Client) IssueTx(tx *chain.Transaction) error {
	c.wl.Lock()
	defer c.wl.Unlock()

	return c.conn.WriteMessage(websocket.BinaryMessage, tx.Bytes())
}

// ListenForTx listens for responses from the streamingServer.
func (c *Client) ListenForTx() (ids.ID, error, *chain.Result, error) {
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

// Close closes [c]'s connection to the decision rpc server.
func (c *Client) Close() error {
	var err error
	c.cl.Do(func() {
		err = c.conn.Close()
	})
	return err
}

// Listen listens for block messages from the streaming server.
func (c *Client) ListenForBlock(
	parser chain.Parser,
) (*chain.StatefulBlock, []*chain.Result, error) {
	c.bll.Lock()
	defer c.bll.Unlock()
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return nil, nil, err
		}
		if msg == nil || msg[0] == BlockMode {
			return UnpackBlockMessageBytes(msg, parser)
		}
	}
}

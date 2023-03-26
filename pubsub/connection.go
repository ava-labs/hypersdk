// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"errors"
	"io"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var (
	ErrFilterNotInitialized = errors.New("filter not initialized")
	ErrAddressLimit         = errors.New("address limit exceeded")
	ErrInvalidFilterParam   = errors.New("invalid bloom filter params")
	ErrInvalidCommand       = errors.New("invalid command")
)

// Callback type is used as a callback function for the
// WebSocket server to process incoming messages.
type Callback func([]byte, []interface{}) []byte

// connection is a representation of the websocket connection.
type connection struct {
	s *Server

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan interface{}

	// Represents if the connection can receive new messages.
	active uint32

	// Callback function when after a server reads from a connection
	rCallback Callback
}

// isActive returns whether the connection is active
func (c *connection) isActive() bool {
	active := atomic.LoadUint32(&c.active)
	return active != 0
}

// deactivate deactivates the connection.
func (c *connection) deactivate() {
	atomic.StoreUint32(&c.active, 0)
}

// Send sends [msg] to c's send channel and returns whether the message was sent.
func (c *connection) Send(msg interface{}) bool {
	if !c.isActive() {
		return false
	}
	select {
	case c.send <- msg:
		return true
	default:
	}
	return false
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *connection) readPump(e []interface{}) {
	defer func() {
		c.deactivate()
		c.s.removeConnection(c)

		// close is called by both the writePump and the readPump so one of them
		// will always error
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	// SetReadDeadline returns an error if the connection is corrupted
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, reader, err := c.conn.NextReader()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				c.s.log.Debug("unexpected close in websockets",
					zap.Error(err),
				)
			}
			break
		}
		if c.rCallback != nil {
			responseBytes, err := io.ReadAll(reader)
			if err == nil {
				c.s.log.Debug("unexpected error reading bytes from websockets",
					zap.Error(err),
				)
			}
			c.Send(c.rCallback(responseBytes, e))
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		c.deactivate()
		ticker.Stop()
		c.s.removeConnection(c)

		// close is called by both the writePump and the readPump so one of them
		// will always error
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.s.log.Debug("closing the connection",
					zap.String("reason", "failed to set the write deadline"),
					zap.Error(err),
				)
				return
			}
			if !ok {
				// The hub closed the channel. Attempt to close the connection
				// gracefully.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.s.log.Debug("closing the connection",
					zap.String("reason", "failed to set the write deadline"),
					zap.Error(err),
				)
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

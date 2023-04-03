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
// Accepts a byte message, the connection and any additional information.
type Callback func([]byte, *Connection) []byte

// connection is a representation of the websocket connection.
type Connection struct {
	s *Server

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// Represents if the connection can receive new messages.
	active atomic.Bool
}

// isActive returns whether the connection is active
func (c *Connection) isActive() bool {
	return c.active.Load()
}

// deactivate deactivates the connection.
func (c *Connection) deactivate() {
	c.active.Store(false)
}

// Send sends [msg] to c's send channel and returns whether the message was sent.
func (c *Connection) Send(msg []byte) bool {
	if !c.isActive() {
		return false
	}
	select {
	case c.send <- msg:
		return true
	default:
		c.s.log.Debug("msg was dropped")
	}
	return false
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Connection) readPump() {
	defer func() {
		c.deactivate()
		c.s.removeConnection(c)

		// close is called by both the writePump and the readPump so one of them
		// will always error
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(c.s.config.MaxMessageSize)
	// SetReadDeadline returns an error if the connection is corrupted
	if err := c.conn.SetReadDeadline(time.Now().Add(c.s.config.PongWait)); err != nil {
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.s.config.PongWait))
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
		if c.s.callback != nil {
			responseBytes, err := io.ReadAll(reader)
			if err == nil {
				c.s.log.Debug("unexpected error reading bytes from websockets",
					zap.Error(err),
				)
			}
			c.Send(c.s.callback(responseBytes, c))
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Connection) writePump() {
	ticker := time.NewTicker(c.s.config.PingPeriod)
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
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.s.config.WriteWait)); err != nil {
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
			if err := c.conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.s.config.WriteWait)); err != nil {
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

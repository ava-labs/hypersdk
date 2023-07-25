// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"io"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Callback type is used as a callback function for the
// WebSocket server to process incoming messages.
// Accepts a byte message, the connection and any additional information.
type Callback func([]byte, *Connection)

// connection is a representation of the websocket connection.
type Connection struct {
	s *Server

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	mb *MessageBuffer

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
	_ = c.mb.Close()
}

// Send sends [msg] to c's send channel and returns whether the message was sent.
func (c *Connection) Send(msg []byte) bool {
	if !c.isActive() {
		return false
	}
	if err := c.mb.Send(msg); err != nil {
		c.s.log.Debug("unable to send message", zap.Error(err))
		return false
	}
	return true
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Connection) readPump() {
	defer func() {
		c.s.removeConnection(c)
		c.deactivate()

		// close is called by both the writePump and the readPump so one of them
		// will always error
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(int64(c.s.config.MaxReadMessageSize))
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
			return
		}
		if c.s.callback == nil {
			continue
		}
		responseBytes, err := io.ReadAll(reader)
		if err != nil {
			c.s.log.Debug("unexpected error reading bytes from websockets",
				zap.Error(err),
			)
			return
		}
		msgs, err := ParseBatchMessage(c.s.config.MaxReadMessageSize, responseBytes)
		if err != nil {
			c.s.log.Debug("unable to read websockets message",
				zap.Error(err),
			)
			return
		}
		for _, msg := range msgs {
			c.s.callback(msg, c)
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
		c.s.removeConnection(c)
		c.deactivate()
		ticker.Stop()

		// close is called by both the writePump and the readPump so one of them
		// will always error
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.mb.Queue:
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
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
				c.s.log.Debug("closing the connection",
					zap.String("reason", "failed to write message"),
					zap.Error(err),
				)
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

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Callback type is used as a callback function for the
// WebSocket server to process incoming messages.
// Accepts a byte message, the connection and any additional information.
type Callback func(uint64, []byte, *Connection)

// connection is a representation of the websocket connection.
type Connection struct {
	s *Server

	// The websocket connection.
	conn     *websocket.Conn
	received uint64

	// Buffered channel of outbound messages.
	mb *MessageBuffer

	// Represents if the connection can receive new messages.
	active atomic.Bool
	closer sync.Once
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
	if _, err := c.mb.Send(msg); err != nil {
		c.s.log.Debug("unable to send message", zap.Error(err))
		return false
	}
	return true
}

func (c *Connection) cleanup() {
	c.closer.Do(func() {
		c.s.removeConnection(c)
		c.deactivate()
	})
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Connection) readPump() {
	defer c.cleanup()

	// Ensure connection stays open as long as we get pings
	if err := c.conn.SetReadDeadline(time.Now().Add(c.s.config.PongWait)); err != nil {
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.s.config.PongWait))
	})

	c.conn.SetReadLimit(int64(c.s.config.MaxReadMessageSize))
	for {
		messageType, response, err := c.conn.ReadMessage()
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
		if messageType != websocket.BinaryMessage {
			c.s.log.Debug("dropping non-binary message", zap.Int("messageType", messageType))
			continue
		}
		if c.s.callback == nil {
			continue
		}
		if len(response) == 0 {
			// TODO: disconnect here?
			c.s.log.Debug("dropping empty message")
			continue
		}
		msgs, err := ParseBatchMessage(c.s.config.MaxReadMessageSize, response)
		if err != nil {
			c.s.log.Debug("unable to read websockets message",
				zap.Error(err),
			)
			return
		}
		for _, msg := range msgs {
			c.s.callback(c.received, msg, c)
			c.received++
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Connection) writePump() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()

		_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
		_ = c.conn.Close()

		c.cleanup()
	}()

	for {
		select {
		case message, ok := <-c.mb.Queue:
			if !ok {
				return
			}
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.s.config.WriteWait)); err != nil {
				c.s.log.Debug("closing the connection",
					zap.String("reason", "failed to set the write deadline"),
					zap.Error(err),
				)
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
				c.s.log.Debug("closing the connection",
					zap.String("reason", "failed to write ping"),
					zap.Error(err),
				)
				return
			}
		}
	}
}

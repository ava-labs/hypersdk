// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type ServerConfig struct {
	// Size of the ws read buffer
	ReadBufferSize int
	// Size of the ws write buffer
	WriteBufferSize int
	// Maximum number of pending messages to send to a peer.
	MaxPendingMessages int
	// Maximum message size in bytes allowed from peer.
	MaxReadMessageSize int
	// Maximum message size in bytes to send to peer.
	MaxWriteMessageSize int
	// Maximum delay for a single message to wait in the buffer
	MaxMessageWait time.Duration
	// Time allowed to write a message to the peer.
	WriteWait time.Duration
	// Time allowed to read the next pong message from the peer.
	PongWait time.Duration
	// Send pings to peer with this period. Must be less than pongWait.
	PingPeriod time.Duration
}

func NewDefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ReadBufferSize:      ReadBufferSize,
		WriteBufferSize:     WriteBufferSize,
		MaxPendingMessages:  MaxPendingMessages,
		MaxReadMessageSize:  MaxReadMessageSize,
		MaxWriteMessageSize: MaxWriteMessageSize,
		MaxMessageWait:      MaxMessageWait,
		WriteWait:           WriteWait,
		PongWait:            PongWait,
		PingPeriod:          (9 * PongWait) / 10,
	}
}

// Server maintains the set of active clients and sends messages to the clients.
//
// Connect to the server after starting using websocket.DefaultDialer.Dial().
type Server struct {
	log      logging.Logger
	config   *ServerConfig
	callback Callback
	upgrader *websocket.Upgrader
	conns    *Connections
}

// New returns a new Server instance. The callback function [f] is called
// by the server in response to messages if not nil.
func New(
	log logging.Logger,
	config *ServerConfig,
	callback Callback,
) *Server {
	return &Server{
		log:      log,
		config:   config,
		callback: callback,
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool {
				return true
			},
			ReadBufferSize:  config.ReadBufferSize,
			WriteBufferSize: config.WriteBufferSize,
		},
		conns: NewConnections(),
	}
}

// ServeHTTP adds a connection to the server, and starts go routines for
// reading and writing.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Upgrader.upgrade() is called to upgrade the HTTP connection.
	// No nead to set any headers so we pass nil as the last argument.
	wsConn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("failed to upgrade",
			zap.Error(err),
		)
		return
	}
	s.addConnection(&Connection{
		s:      s,
		conn:   wsConn,
		mb:     NewMessageBuffer(s.log, s.config.MaxPendingMessages, s.config.MaxWriteMessageSize, s.config.MaxMessageWait),
		active: atomic.Bool{},
	})
	s.log.Debug("added pubsub connection", zap.Stringer("addr", wsConn.RemoteAddr()))
}

// Publish sends msg from [s] to [toConns].
func (s *Server) Publish(msg []byte, conns *Connections) []*Connection {
	inactiveConnections := []*Connection{}
	for _, conn := range conns.Conns() {
		if !s.conns.Has(conn) {
			inactiveConnections = append(inactiveConnections, conn)
			continue
		}
		if !conn.Send(msg) {
			s.log.Verbo(
				"dropping message to subscribed connection due to too many pending messages",
			)
		}
	}
	return inactiveConnections
}

// addConnection adds [conn] to the servers connection set and starts go
// routines for reading and writing messages for the connection.
func (s *Server) addConnection(conn *Connection) {
	conn.active.Store(true)
	s.conns.Add(conn)

	go conn.writePump()
	go conn.readPump()
}

// removeConnection removes [conn] from the servers connection set.
func (s *Server) removeConnection(conn *Connection) {
	s.conns.Remove(conn)
}

func (s *Server) Connections() *Connections {
	return s.conns
}

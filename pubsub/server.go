// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/utils/logging"
)

type ServerConfig struct {
	// Size of the ws read buffer
	ReadBufferSize int
	// Size of the ws write buffer
	WriteBufferSize int
	// Maximum number of pending messages to send to a peer.
	MaxPendingMessages int
	// Maximum message size in bytes allowed from peer.
	MaxMessageSize int64
	// Time allowed to write a message to the peer.
	WriteWait time.Duration
	// Time allowed to read the next pong message from the peer.
	PongWait time.Duration
	// Send pings to peer with this period. Must be less than pongWait.
	PingPeriod time.Duration
	// ReadHeaderTimeout is the maximum duration for reading a request.
	ReadHeaderTimeout time.Duration
}

func NewDefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ReadBufferSize:     ReadBufferSize,
		WriteBufferSize:    WriteBufferSize,
		MaxPendingMessages: MaxPendingMessages,
		MaxMessageSize:     MaxMessageSize,
		WriteWait:          WriteWait,
		PongWait:           PongWait,
		PingPeriod:         (9 * PongWait) / 10,
		ReadHeaderTimeout:  ReadHeaderTimeout,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

// Server maintains the set of active clients and sends messages to the clients.
//
// Connect to the server after starting using websocket.DefaultDialer.Dial().
type Server struct {
	// The http server
	s *http.Server
	// The address to listen on
	addr string
	log  logging.Logger
	lock sync.RWMutex
	// conns a set of all our connections
	conns *Connections
	// Callback function when server receives a message
	callback Callback
	// Config variables
	config *ServerConfig
}

// New returns a new Server instance. The callback function [f] is called
// by the server in response to messages if not nil.
func New(
	addr string,
	r Callback,
	log logging.Logger,
	config *ServerConfig,
) *Server {
	upgrader.ReadBufferSize = config.ReadBufferSize
	upgrader.WriteBufferSize = config.WriteBufferSize
	return &Server{
		log:      log,
		addr:     addr,
		callback: r,
		conns:    NewConnections(),
		config:   config,
	}
}

// ServeHTTP adds a connection to the server, and starts go routines for
// reading and writing.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Upgrader.upgrade() is called to upgrade the HTTP connection.
	// No nead to set any headers so we pass nil as the last argument.
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Debug("failed to upgrade",
			zap.Error(err),
		)
		return
	}
	s.addConnection(&Connection{
		s:      s,
		conn:   wsConn,
		send:   make(chan []byte, s.config.MaxPendingMessages),
		active: atomic.Bool{},
	})
}

// Publish sends msg from [s] to [toConns].
func (s *Server) Publish(msg []byte, toConns *Connections) {
	for _, conn := range toConns.Conns() {
		// check server has connection O(1)
		if !s.conns.Has(conn) {
			continue
		}
		if !conn.Send(msg) {
			s.log.Verbo(
				"dropping message to subscribed connection due to too many pending messages",
			)
		}
	}
}

// addConnection adds [conn] to the servers connection set and starts go
// routines for reading and writing messages for the connection.
func (s *Server) addConnection(conn *Connection) {
	s.lock.Lock()
	defer s.lock.Unlock()

	conn.active.Store(true)
	s.conns.Add(conn)

	go conn.writePump()
	go conn.readPump()
}

// removeConnection removes [conn] from the servers connection set.
func (s *Server) removeConnection(conn *Connection) {
	s.conns.Remove(conn)
}

// Start starts the server. Returns an error if the server fails to start or
// when the server is stopped.
func (s *Server) Start() error {
	s.lock.Lock()
	s.s = &http.Server{
		Addr:              s.addr,
		Handler:           s,
		ReadHeaderTimeout: s.config.ReadHeaderTimeout,
	}
	s.lock.Unlock()
	err := s.s.ListenAndServe()
	return err
}

// Shutdown shuts down the server and returns the associated error.
func (s *Server) Shutdown(c context.Context) error {
	return s.s.Shutdown(c)
}

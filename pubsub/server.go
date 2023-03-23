// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
)

const (
	// Size of the ws read buffer
	readBufferSize = units.KiB

	// Size of the ws write buffer
	writeBufferSize = units.KiB

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 3 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 10 * units.KiB // bytes

	// Maximum number of pending messages to send to a peer.
	maxPendingMessages = 1024 // messages

	// MaxBytes the max number of bytes for a filter
	MaxBytes = 1 * units.MiB

	// MaxAddresses the max number of addresses allowed
	MaxAddresses = 10000
)

type errorMsg struct {
	Error string `json:"error"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  readBufferSize,
	WriteBufferSize: writeBufferSize,
	CheckOrigin: func(*http.Request) bool {
		return true
	},
}

// Server maintains the set of active clients and sends messages to the clients.
type Server struct {
	log  logging.Logger
	lock sync.RWMutex
	// conns a list of all our connections
	conns set.Set[*connection]
	// subscribedConnections the connections that have activated subscriptions
	subscribedConnections *connections
}

// New returns a new Server instance with the logger set to [log].
func New(log logging.Logger) *Server {
	return &Server{
		log:                   log,
		subscribedConnections: newConnections(),
	}
}

// ServeHTTP adds a connection
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Debug("failed to upgrade",
			zap.Error(err),
		)
		return
	}
	conn := &connection{
		s:      s,
		conn:   wsConn,
		send:   make(chan interface{}, maxPendingMessages),
		active: 1,
	}
	s.addConnection(conn)
}

// Publish sends msg from [parser] to the relevant servers subscribed connections
// filtered by [parser].
func (s *Server) Publish(msg interface{}) {
	conns := s.subscribedConnections.Conns()
	for i := range conns {
		conn := conns[i]
		if !conn.Send(msg) {
			s.log.Verbo(
				"dropping message to subscribed connection due to too many pending messages",
			)
		}
	}
}

// addConnection adds [conn] to the servers connection set and starts go
// routines for reading and writing messages for the connection.
func (s *Server) addConnection(conn *connection) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.conns.Add(conn)

	go conn.writePump()
	go conn.readPump()
}

// removeConnection removes [conn] from the servers connection set.
func (s *Server) removeConnection(conn *connection) {
	s.subscribedConnections.Remove(conn)

	s.lock.Lock()
	defer s.lock.Unlock()

	s.conns.Remove(conn)
}

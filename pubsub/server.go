// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/utils/logging"
)

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
	// conns a set of all our connections
	conns Connections
	// Callback function when server receives a message
	rCallback Callback
	//  Supplemental info for the rCallback function
	e []interface{}
}

// New returns a new Server instance with the logger set to [log]. The callback
// function [f] is called by the server in response to messages if not nil.
func New(log logging.Logger, r Callback, e []interface{}) *Server {
	return &Server{
		log:       log,
		rCallback: r,
		conns:     *newConnections(),
		e:         e,
	}
}

// ServeHTTP adds a connection to the server, and starts go routines for
// reading and writing.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Debug("failed to upgrade",
			zap.Error(err),
		)
		return
	}
	conn := &Connection{
		s:         s,
		conn:      wsConn,
		send:      make(chan interface{}, maxPendingMessages),
		active:    1,
		rCallback: s.rCallback,
	}
	s.addConnection(conn)
}

// Publish sends msg from [s] to [toConns].
func (s *Server) Publish(msg interface{}, toConns *Connections) {
	for _, conn := range toConns.Conns() {
		// check server has connection O(1)
		if !s.conns.conns.Contains(conn) {
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

	s.conns.Add(conn)

	go conn.writePump()
	go conn.readPump(s.e)
}

// removeConnection removes [conn] from the servers connection set.
func (s *Server) removeConnection(conn *Connection) {
	s.conns.Remove(conn)
}

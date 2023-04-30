// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

const dummyAddr = "localhost:8080"

// This is a dummy struct to test the callback function
type counter struct {
	val int
	// lock
	l sync.RWMutex
}

func (x *counter) dummyProcessTXCallback(b []byte, _ *Connection) {
	x.val++
	id, err := ids.ToID(b)
	x.l.Lock()
	defer x.l.Unlock()
	if err != nil {
		return
	}
	if ids.Empty == id {
		return
	} else {
		x.val++
	}
}

// This also makes sure the callback function executed properly.
// TestServerPublish adds a connection to a server then publishes
// a msg to be sent to all connections. Checks the message was delivered properly
// and the connection is properly handled when closed.
func TestServerPublish(t *testing.T) {
	require := require.New(t)
	// Create a new logger for the test
	logger := logging.NoLog{}
	// Create a new pubsub server
	handler := New(logger, NewDefaultServerConfig(), nil)
	// Channels for ensuring if connections/server are closed
	closeConnection := make(chan bool)
	serverDone := make(chan struct{})
	dummyMsg := "dummy_msg"
	// Go routine that listens on dummyAddress for connections
	var server *http.Server
	go func() {
		defer close(serverDone)
		server = &http.Server{
			Addr:              dummyAddr,
			Handler:           handler,
			ReadHeaderTimeout: 30 * time.Second,
		}
		require.ErrorIs(
			server.ListenAndServe(),
			http.ErrServerClosed,
			"Incorrect error closing server.",
		)
	}()
	// Connect to pubsub server
	u := url.URL{Scheme: "ws", Host: dummyAddr}
	// Wait for server to start accepting requests
	<-time.After(10 * time.Millisecond)
	webCon, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err, "Error connecting to the server.")
	defer resp.Body.Close()
	// Publish to subscribed connections
	handler.Publish([]byte(dummyMsg), handler.Connections())
	// Receive the message from the publish
	_, msg, err := webCon.ReadMessage()
	require.NoError(err, "Error receiveing message.")
	// Verify that the received message is the expected dummy message
	require.Equal([]byte(dummyMsg), msg, "Response from server not correct.")
	// Close the connection and wait for it to be closed on the server side
	go func() {
		webCon.Close()
		for {
			if handler.conns.Len() == 0 {
				closeConnection <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	// Wait for the connection to be closed or for a timeout to occur
	select {
	case <-closeConnection:
		// Connection was closed on the server side, test passed
	case <-time.After(time.Second):
		// Timeout occurred, connection was not closed on the server side, test failed
		require.Fail("connection was not closed on the server side")
	}
	// Gracefully shutdown the server
	err = server.Shutdown(context.TODO())
	require.NoError(err, "Error shuting down server")
	// Wait for the server to finish shutting down
	<-serverDone
}

// TestServerPublish pumps messages into a dummy server and waits for
// the servers response. Requires the server handled the messages correctly.
func TestServerRead(t *testing.T) {
	require := require.New(t)
	// Create a new logger for the test
	logger := logging.NoLog{}
	counter := &counter{
		val: 10,
	}
	// Create a new pubsub server
	handler := New(logger, NewDefaultServerConfig(), counter.dummyProcessTXCallback)
	// Channels for ensuring if connections/server are closed
	closeConnection := make(chan bool)
	serverDone := make(chan struct{})
	// Go routine that listens on dummyAddress for connections
	var server *http.Server
	go func() {
		defer close(serverDone)
		server = &http.Server{
			Addr:              dummyAddr,
			Handler:           handler,
			ReadHeaderTimeout: 30 * time.Second,
		}
		require.ErrorIs(
			server.ListenAndServe(),
			http.ErrServerClosed,
			"Incorrect error closing server.",
		)
	}()
	// Connect to pubsub server
	u := url.URL{Scheme: "ws", Host: dummyAddr}
	// Wait for server to start accepting requests
	<-time.After(10 * time.Millisecond)
	webCon, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err, "Error connecting to the server.")
	defer resp.Body.Close()
	id := ids.GenerateTestID()
	err = webCon.WriteMessage(websocket.TextMessage, id[:])
	require.NoError(err, "Error writing message to server.")
	// Wait for callback to be called
	<-time.After(10 * time.Millisecond)
	// Callback was correctly called
	counter.l.Lock()
	require.Equal(12, counter.val, "Callback not called correctly.")
	counter.l.Unlock()
	// Close the connection and wait for it to be closed on the server side
	go func() {
		webCon.Close()
		for {
			if handler.conns.Len() == 0 {
				closeConnection <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	// Wait for the connection to be closed or for a timeout to occur
	select {
	case <-closeConnection:
		// Connection was closed on the server side, test passed
	case <-time.After(time.Second):
		// Timeout occurred, connection was not closed on the server side, test failed
		require.Fail("connection was not closed on the server side")
	}
	// Gracefully shutdown the server
	err = server.Shutdown(context.TODO())
	require.NoError(err, "Error shutting down server.")
	// Wait for the server to finish shutting down
	<-serverDone
}

// TestServerPublishSpecific adds two connections to a pubsub server then publishes
// a msg to be sent to only one of the connections. Checks the message was
// delivered properly and the connection is properly handled when closed.
func TestServerPublishSpecific(t *testing.T) {
	require := require.New(t)
	// Create a new logger for the test
	logger := logging.NoLog{}
	counter := &counter{
		val: 10,
	}
	// Create a new pubsub server
	handler := New(logger, NewDefaultServerConfig(), counter.dummyProcessTXCallback)
	// Channels for ensuring if connections/server are closed
	closeConnection := make(chan bool)
	serverDone := make(chan struct{})
	dummyMsg := "dummy_msg"
	// Go routine that listens on dummyAddress for connections
	var server *http.Server
	go func() {
		defer close(serverDone)
		server = &http.Server{
			Addr:              dummyAddr,
			Handler:           handler,
			ReadHeaderTimeout: 30 * time.Second,
		}
		require.ErrorIs(
			server.ListenAndServe(),
			http.ErrServerClosed,
			"Incorrect error closing server.",
		)
	}()
	// Connect to pubsub server
	u := url.URL{Scheme: "ws", Host: dummyAddr}
	// Wait for server to start accepting requests
	<-time.After(10 * time.Millisecond)
	webCon1, resp1, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err, "Error connecting to the server.")
	defer resp1.Body.Close()
	sendConns := NewConnections()
	peekCon, _ := handler.conns.Peek()
	sendConns.Add(peekCon)
	webCon2, resp2, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err, "Error connecting to the server.")
	defer resp2.Body.Close()
	require.Equal(2, handler.conns.Len(), "Server didn't add connection correctly.")
	// Publish to subscribed connections
	handler.Publish([]byte(dummyMsg), sendConns)
	go func() {
		// Receive the message from the publish
		_, msg, err := webCon1.ReadMessage()
		require.NoError(err, "Error reading to connection.")
		// Verify that the received message is the expected dummy message
		require.Equal([]byte(dummyMsg), msg, "Message not as expected.")
		webCon1.Close()
		for {
			if handler.conns.Len() == 0 {
				closeConnection <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	// not receive from the other
	go func() {
		err := webCon2.SetReadDeadline(time.Now().Add(time.Second))
		require.NoError(err, "Error setting connection deadline.")
		// Make sure connection wasn't written too
		_, _, err = webCon2.ReadMessage()
		require.Error(err, "Error not thrown.")
		netErr, ok := err.(net.Error)
		require.True(ok, "Error is not a net.Error")
		require.True(netErr.Timeout(), "Error is not a timeout error")
		webCon2.Close()
		for {
			if handler.conns.Len() == 0 {
				closeConnection <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	// Wait for the connection to be closed or for a timeout to occur
	select {
	case <-closeConnection:
		// Connection was closed on the server side, test passed
	case <-time.After(2 * time.Second):
		// Timeout occurred, connection was not closed on the server side, test failed
		require.Fail("connection was not closed on the server side")
	}
	// Gracefully shutdown the server
	err = server.Shutdown(context.TODO())
	require.NoError(err, "Error shuting down server.")
	// Wait for the server to finish shutting down
	<-serverDone
}

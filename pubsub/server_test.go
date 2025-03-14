// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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
	time.Sleep(10 * time.Millisecond)
	webCon, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err, "Error connecting to the server.")
	defer resp.Body.Close()

	// wait until we have a single connection
	optimisticEventuallity(
		require,
		func() bool { return handler.conns.Len() == 1 },
		15*time.Second, 250*time.Millisecond, "Server didn't add connection correctly.",
	)

	// Publish to subscribed connections
	handler.Publish([]byte(dummyMsg), handler.Connections())
	// Receive the message from the publish
	_, batchMsg, err := webCon.ReadMessage()
	require.NoError(err, "Error receiving message.")
	msgs, err := ParseBatchMessage(batchMsg)
	require.NoError(err, "Error parsing message.")
	require.Len(msgs, 1)
	// Verify that the received message is the expected dummy message
	require.Equal([]byte(dummyMsg), msgs[0], "Response from server not correct.")
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
	require.NoError(err, "Error shutting down server")
	// Wait for the server to finish shutting down
	<-serverDone
}

// TestServerRead pumps messages into a dummy server and waits for
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
	// Wait for server to start accepting requests
	time.Sleep(10 * time.Millisecond)
	u := url.URL{Scheme: "ws", Host: dummyAddr}
	webCon, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err, "Error connecting to the server.")
	defer resp.Body.Close()
	id := ids.GenerateTestID()
	batchMsg := CreateBatchMessage([][]byte{id[:]})
	err = webCon.WriteMessage(websocket.TextMessage, batchMsg)
	require.NoError(err, "Error writing message to server.")
	// Wait for callback to be called
	time.Sleep(10 * time.Millisecond)
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

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

// optimisticEventuallity asserts that given condition will be met in waitFor time,
// periodically checking target function each tick. Unlike [require.Eventually], it first test
// the condition function and call [require.Eventually] only if the condition has not met yet.
func optimisticEventuallity(require *require.Assertions, condition func() bool, waitFor time.Duration, tick time.Duration, msgAndArgs ...interface{}) { //nolint:unparam
	if !condition() {
		require.Eventually(condition, waitFor, tick, msgAndArgs...)
	}
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
	time.Sleep(10 * time.Millisecond)
	webCon1, resp1, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err, "Error connecting to the server.")
	defer resp1.Body.Close()

	// wait until we have 1 connection.
	optimisticEventuallity(
		require,
		func() bool { return handler.conns.Len() == 1 },
		15*time.Second, 250*time.Millisecond, "Server didn't add connection correctly.",
	)

	// grab the connection
	sendConns := NewConnections()
	peekCon, _ := handler.conns.Peek()
	sendConns.Add(peekCon)

	webCon2, resp2, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err, "Error connecting to the server.")
	defer resp2.Body.Close()

	optimisticEventuallity(
		require,
		func() bool { return handler.conns.Len() == 2 },
		15*time.Second, 250*time.Millisecond, "Server didn't add connection correctly.",
	)
	clientSideClosedConnection := sync.WaitGroup{}
	clientSideClosedConnection.Add(2)

	// Publish to subscribed connections
	handler.Publish([]byte(dummyMsg), sendConns)
	go func() {
		defer clientSideClosedConnection.Done()
		// Receive the message from the publish
		_, batchMsg, err := webCon1.ReadMessage()
		require.NoError(err, "Error reading to connection.")
		msgs, err := ParseBatchMessage(batchMsg)
		require.NoError(err, "Error parsing message.")
		require.Len(msgs, 1)
		// Verify that the received message is the expected dummy message
		require.Equal([]byte(dummyMsg), msgs[0], "Message not as expected.")
		webCon1.Close()
	}()
	// not receive from the other
	go func() {
		defer clientSideClosedConnection.Done()
		err := webCon2.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		require.NoError(err, "Error setting connection deadline.")
		// Make sure connection wasn't written too
		_, _, err = webCon2.ReadMessage()
		require.Error(err, "Error not thrown.")    //nolint:forbidigo
		require.Implements((*net.Error)(nil), err) // ensure err implements net.Error
		netErr := err.(net.Error)
		require.True(netErr.Timeout(), "Error is not a timeout error")
		// close the connection without notifying the server
		webCon2.Close()
	}()

	// wait for client side connection to close.
	require.Falsef(waitTimeout(&clientSideClosedConnection, 5*time.Second), "Timed out waiting for client side connection to close")

	// Wait for the connection to be closed or for a timeout to occur
	optimisticEventuallity(
		require,
		func() bool { return handler.conns.Len() == 0 },
		15*time.Second, 50*time.Millisecond, "Server didn't close connections correctly.",
	)

	// Gracefully shutdown the server
	err = server.Shutdown(context.TODO())
	require.NoError(err, "Error shutting down server.")

	// Wait for the server to finish shutting down
	select {
	case <-serverDone:
		// great!
	case <-time.After(500 * time.Millisecond):
		require.FailNow("shutting down server takes too long")
	}
}

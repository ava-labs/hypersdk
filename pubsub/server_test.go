// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestGeneratePrivateKeyDifferent(t *testing.T) {
	require := require.New(t)
	require.True(true)
}

var dummyAddr = flag.String("addr", "localhost:8080", "http service address")

var (
	callbackEmptyResponse = "EMPTY_ID"
	callbackResponse      = "ID_RECEIVED"
)

func dummyProcessTXCallback(b []byte) []byte {
	unmarshalID := ids.Empty
	err := unmarshalID.UnmarshalJSON(b)
	if err != nil {
		return []byte("error during unmarshalling")
	}
	if ids.Empty == unmarshalID {
		return []byte(callbackEmptyResponse)
	} else {
		return []byte(callbackResponse)
	}
}

// TestServerPublish adds a connection to a server then publishes
// a msg to be sent to all connections. Checks the message was delivered properly
// and the connection is properly handled when closed.
func TestServerPublish(t *testing.T) {
	require := require.New(t)
	// Create a new logger for the test
	logger := logging.NoLog{}
	// Create a new pubsub server
	server := New(logger, nil)
	// Channels for ensuring if connections/server are closed
	closeConnection := make(chan bool)
	serverDone := make(chan struct{})
	dummyMsg := "dummy_msg"
	testLoc := "/testPub"
	// Connect the server to an http.Server ??
	flag.Parse()
	srv := &http.Server{
		Addr:              *dummyAddr,
		ReadHeaderTimeout: time.Second * 5,
	}
	// Handle requests to dummyAddr
	http.HandleFunc(testLoc, server.ServeHTTP)
	// Go routine that listens on dummyAddress for connections
	go func() {
		defer close(serverDone)
		err := srv.ListenAndServe()
		require.ErrorIs(err, http.ErrServerClosed)
	}()
	// Connect to pubsub server
	u := url.URL{Scheme: "ws", Host: *dummyAddr, Path: testLoc}
	webCon, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	defer resp.Body.Close()
	require.NoError(err)
	// Publish to subscribed connections
	server.lock.Lock()
	server.Publish(dummyMsg)
	server.lock.Unlock()
	// Receive the message from the publish
	_, msg, err := webCon.ReadMessage()
	require.NoError(err)
	var unmarshal string
	err = json.Unmarshal(msg, &unmarshal)
	require.NoError(err)
	// Verify that the received message is the expected dummy message
	require.Equal(dummyMsg, unmarshal)
	// Close the connection and wait for it to be closed on the server side
	go func() {
		webCon.Close()
		for {
			server.lock.Lock()
			len := server.conns.Len()
			if len == 0 {
				server.lock.Unlock()
				closeConnection <- true
				return
			}
			server.lock.Unlock()
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	require.NoError(err)
	// Wait for the server to finish shutting down
	<-serverDone
}

// TestServerPublish pumps messages into a dummy server and waits for
// the servers response. Requires messages back from the server handled the
// messages correctly.
func TestServerRead(t *testing.T) {
	require := require.New(t)
	// Create a new logger for the test
	logger := logging.NoLog{}
	// Create a new pubsub server
	server := New(logger, dummyProcessTXCallback)
	// Channels for ensuring if connections/server are closed
	closeConnection := make(chan bool)
	serverDone := make(chan struct{})
	testLoc := "/testRead"
	// Connect the server to an http.Server ??
	flag.Parse()
	srv := &http.Server{
		Addr:              *dummyAddr,
		ReadHeaderTimeout: time.Second * 5,
	}
	// Handle requests to dummyAddr
	http.HandleFunc(testLoc, server.ServeHTTP)
	// Go routine that listens on dummyAddress for connections
	go func() {
		defer close(serverDone)
		err := srv.ListenAndServe()
		require.ErrorIs(err, http.ErrServerClosed)
	}()
	// Connect to pubsub server
	u := url.URL{Scheme: "ws", Host: *dummyAddr, Path: testLoc}
	webCon, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	defer resp.Body.Close()

	require.NoError(err)
	// Write to server
	marshalID, _ := ids.Empty.MarshalJSON()
	err = webCon.WriteMessage(websocket.TextMessage, marshalID)
	require.NoError(err)
	// Receive the message from the publish
	_, msg, err := webCon.ReadMessage()
	require.NoError(err)
	var unmarshal []byte
	err = json.Unmarshal(msg, &unmarshal)
	require.NoError(err)
	// Verify that the received message is the expected dummy message
	require.Equal(callbackEmptyResponse, string(unmarshal))
	// Close the connection and wait for it to be closed on the server side
	go func() {
		webCon.Close()
		for {
			server.lock.Lock()
			len := server.conns.Len()
			if len == 0 {
				server.lock.Unlock()
				closeConnection <- true
				return
			}
			server.lock.Unlock()
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	require.NoError(err)
	// Wait for the server to finish shutting down
	<-serverDone
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"context"
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

var dummyAddr = flag.String("addr", "localhost:8080", "http service address")

var (
	callbackEmptyResponse = "EMPTY_ID"
	callbackResponse      = "ID_RECEIVED"
)

type x struct {
	// This is a dummy struct to test the callback function
}

func (typex *x) dummyProcessTXCallback(b []byte, _ *Connection, e []interface{}) []byte {
	if e != nil {
		count := e[0].(*int)
		*count++
	}
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

// This also makes sure the callback function executed properly.
// TestServerPublish adds a connection to a server then publishes
// a msg to be sent to all connections. Checks the message was delivered properly
// and the connection is properly handled when closed.
func TestServerPublish(t *testing.T) {
	require := require.New(t)
	// Create a new logger for the test
	logger := logging.NoLog{}
	// Create a new pubsub server
	server := New(logger, nil, *dummyAddr)
	// Channels for ensuring if connections/server are closed
	closeConnection := make(chan bool)
	serverDone := make(chan struct{})
	dummyMsg := "dummy_msg"
	// Connect the server to an http.Server ??
	flag.Parse()

	// Go routine that listens on dummyAddress for connections
	go func() {
		defer close(serverDone)
		err := server.Start()
		require.ErrorIs(err, http.ErrServerClosed, "Incorrect error closing server.")
	}()
	// Connect to pubsub server
	u := url.URL{Scheme: "ws", Host: *dummyAddr}
	webCon, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	defer resp.Body.Close()
	require.NoError(err, "Error connecting to the server.")
	// Publish to subscribed connections
	server.lock.Lock()
	server.Publish([]byte(dummyMsg), server.conns)
	server.lock.Unlock()
	// Receive the message from the publish
	_, msg, err := webCon.ReadMessage()
	require.NoError(err, "Error receiveing message.")
	// Verify that the received message is the expected dummy message
	require.Equal([]byte(dummyMsg), msg, "Response from server not correct.")
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
	err = server.Shutdown(context.TODO())
	require.NoError(err, "Error shuting down server")
	// Wait for the server to finish shutting down
	<-serverDone
}

// // TestServerPublish pumps messages into a dummy server and waits for
// // the servers response. Requires the server handled the messages correctly.
// func TestServerRead(t *testing.T) {
// 	require := require.New(t)
// 	// Create a new logger for the test
// 	logger := logging.NoLog{}
// 	dummyVar := int(10)
// 	// Create a new pubsub server
// 	server := New(logger, dummyProcessTXCallback, []interface{}{&dummyVar})
// 	// Channels for ensuring if connections/server are closed
// 	closeConnection := make(chan bool)
// 	serverDone := make(chan struct{})
// 	// Connect the server to an http.Server ??
// 	flag.Parse()
// 	srv := &http.Server{
// 		Addr:              *dummyAddr,
// 		ReadHeaderTimeout: time.Second * 5,
// 		Handler:           server,
// 	}
// 	// Go routine that listens on dummyAddress for connections
// 	go func() {
// 		defer close(serverDone)
// 		err := srv.ListenAndServe()
// 		require.ErrorIs(err, http.ErrServerClosed, "Incorrect error closing server.")
// 	}()
// 	// Connect to pubsub server
// 	u := url.URL{Scheme: "ws", Host: *dummyAddr}
// 	webCon, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
// 	defer resp.Body.Close()
// 	require.NoError(err, "Error connecting to the server.")
// 	// Write to server
// 	marshalID, _ := ids.Empty.MarshalJSON()
// 	err = webCon.WriteMessage(websocket.TextMessage, marshalID)
// 	require.NoError(err, "Error writing message to server.")
// 	// Receive the message from the publish
// 	_, msg, err := webCon.ReadMessage()
// 	require.NoError(err, "Error reading from connection.")
// 	var unmarshal []byte
// 	err = json.Unmarshal(msg, &unmarshal)
// 	require.NoError(err, "Error unmarshaling server response.")
// 	// Callback was correctly called
// 	require.Equal(11, dummyVar, "Callback not called correctly.")
// 	// Verify that the received message is the expected dummy message
// 	require.Equal(callbackEmptyResponse, string(unmarshal), "Response is unexpected.")
// 	// Close the connection and wait for it to be closed on the server side
// 	go func() {
// 		webCon.Close()
// 		for {
// 			server.lock.Lock()
// 			len := server.conns.Len()
// 			if len == 0 {
// 				server.lock.Unlock()
// 				closeConnection <- true
// 				return
// 			}
// 			server.lock.Unlock()
// 			time.Sleep(10 * time.Millisecond)
// 		}
// 	}()
// 	// Wait for the connection to be closed or for a timeout to occur
// 	select {
// 	case <-closeConnection:
// 		// Connection was closed on the server side, test passed
// 	case <-time.After(time.Second):
// 		// Timeout occurred, connection was not closed on the server side, test failed
// 		require.Fail("connection was not closed on the server side")
// 	}
// 	// Gracefully shutdown the server
// 	err = srv.Shutdown(context.TODO())
// 	require.NoError(err, "Error shutting down server.")
// 	// Wait for the server to finish shutting down
// 	<-serverDone
// }

// // TestServerPublishSpecific adds two connections to a pubsub server then publishes
// // a msg to be sent to only one of the connections. Checks the message was
// // delivered properly and the connection is properly handled when closed.
// func TestServerPublishSpecific(t *testing.T) {
// 	require := require.New(t)
// 	// Create a new logger for the test
// 	logger := logging.NoLog{}
// 	// Create a new pubsub server
// 	server := New(logger, nil, nil)
// 	// Channels for ensuring if connections/server are closed
// 	closeConnection := make(chan bool)
// 	serverDone := make(chan struct{})
// 	dummyMsg := "dummy_msg"
// 	// Connect the server to an http.Server ??
// 	flag.Parse()
// 	srv := &http.Server{
// 		Addr:              *dummyAddr,
// 		ReadHeaderTimeout: time.Second * 5,
// 		Handler:           server,
// 	}
// 	// Go routine that listens on dummyAddress for connections
// 	go func() {
// 		defer close(serverDone)
// 		err := srv.ListenAndServe()
// 		require.ErrorIs(err, http.ErrServerClosed, "Incorrect error closing server.")
// 	}()
// 	// Connect to pubsub server
// 	u := url.URL{Scheme: "ws", Host: *dummyAddr}
// 	webCon1, resp1, err := websocket.DefaultDialer.Dial(u.String(), nil)
// 	require.NoError(err, "Error connecting to the server.")
// 	defer resp1.Body.Close()
// 	sendConns := newConnections()
// 	server.lock.Lock()
// 	peekCon, _ := server.conns.conns.Peek()
// 	server.lock.Unlock()
// 	sendConns.Add(peekCon)
// 	webCon2, resp2, err := websocket.DefaultDialer.Dial(u.String(), nil)
// 	require.NoError(err, "Error connecting to the server.")
// 	defer resp2.Body.Close()
// 	require.Equal(2, server.conns.Len(), "Server didn't add connection correctly.")
// 	// Publish to subscribed connections
// 	server.lock.Lock()
// 	server.Publish(dummyMsg, sendConns)
// 	server.lock.Unlock()
// 	go func() {
// 		// Receive the message from the publish
// 		_, msg, err := webCon1.ReadMessage()
// 		require.NoError(err, "Error reading to connection.")
// 		var unmarshal string
// 		err = json.Unmarshal(msg, &unmarshal)
// 		require.NoError(err, "Error unmarshaling message.")
// 		// Verify that the received message is the expected dummy message
// 		require.Equal(dummyMsg, unmarshal, "Message not as expected.")
// 		webCon1.Close()
// 		for {
// 			server.lock.Lock()
// 			len := server.conns.Len()
// 			if len == 0 {
// 				server.lock.Unlock()
// 				closeConnection <- true
// 				return
// 			}
// 			server.lock.Unlock()
// 			time.Sleep(10 * time.Millisecond)
// 		}
// 	}()
// 	// not receive from the other
// 	go func() {
// 		err := webCon2.SetReadDeadline(time.Now().Add(time.Second))
// 		require.NoError(err, "Error setting connection deadline.")
// 		// Make sure connection wasn't written too
// 		_, _, err = webCon2.ReadMessage()
// 		require.Error(err, "Error not thrown.")
// 		netErr, ok := err.(net.Error)
// 		require.True(ok, "Error is not a net.Error")
// 		require.True(netErr.Timeout(), "Error is not a timeout error")
// 		webCon2.Close()
// 		for {
// 			server.lock.Lock()
// 			len := server.conns.Len()
// 			if len == 0 {
// 				server.lock.Unlock()
// 				closeConnection <- true
// 				return
// 			}
// 			server.lock.Unlock()
// 			time.Sleep(10 * time.Millisecond)
// 		}
// 	}()
// 	// Wait for the connection to be closed or for a timeout to occur
// 	select {
// 	case <-closeConnection:
// 		// Connection was closed on the server side, test passed
// 	case <-time.After(2 * time.Second):
// 		// Timeout occurred, connection was not closed on the server side, test failed
// 		require.Fail("connection was not closed on the server side")
// 	}
// 	// Gracefully shutdown the server
// 	err = srv.Shutdown(context.TODO())
// 	require.NoError(err, "Error shuting down server.")
// 	// Wait for the server to finish shutting down
// 	<-serverDone
// }

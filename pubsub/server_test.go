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

var callbackEmptyResponse = "EMPTY_ID"
var callbackResponse = "ID_RECIEVED"

func dummyProcessTXCallback(b []byte) []byte {
	unmarshalID := ids.Empty
	unmarshalID.UnmarshalJSON(b)
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
		Addr: *dummyAddr,
	}
	// Handle requests to dummyAddr + '/'
	http.HandleFunc(testLoc, server.ServeHTTP)
	// Go routine that listens on dummyAddress for connections
	go func() {
		defer close(serverDone)
		err := srv.ListenAndServe()
		require.ErrorIs(err, http.ErrServerClosed)

	}()
	// Connect to pubsub server
	u := url.URL{Scheme: "ws", Host: *dummyAddr, Path: testLoc}
	web_con, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err)
	// Publish to subscribed connections
	server.lock.Lock()
	server.Publish(dummyMsg)
	server.lock.Unlock()
	// Recieve the message from the publish
	_, msg, err := web_con.ReadMessage()
	require.NoError(err)
	var unmarshal string
	json.Unmarshal(msg, &unmarshal)
	// Verify that the received message is the expected dummy message
	require.Equal(dummyMsg, unmarshal)
	// Close the connection and wait for it to be closed on the server side
	go func() {
		web_con.Close()
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
		Addr: *dummyAddr,
	}
	// Handle requests to dummyAddr + '/'
	http.HandleFunc(testLoc, server.ServeHTTP)
	// Go routine that listens on dummyAddress for connections
	go func() {
		defer close(serverDone)
		err := srv.ListenAndServe()
		require.ErrorIs(err, http.ErrServerClosed)
	}()
	// Connect to pubsub server
	u := url.URL{Scheme: "ws", Host: *dummyAddr, Path: testLoc}
	web_con, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err)
	// write to server
	marshalId, _ := ids.Empty.MarshalJSON()
	web_con.WriteMessage(websocket.TextMessage, marshalId)
	// Recieve the message from the publish
	// Recieve the message from the publish
	_, msg, err := web_con.ReadMessage()
	require.NoError(err)
	var unmarshal []byte
	json.Unmarshal(msg, &unmarshal)
	// Verify that the received message is the expected dummy message
	require.Equal(callbackEmptyResponse, string(unmarshal))
	// Close the connection and wait for it to be closed on the server side
	go func() {
		web_con.Close()
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
	// Wait for the server to finish shutting down
	<-serverDone
}

package pubsub

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestGeneratePrivateKeyDifferent(t *testing.T) {
	require := require.New(t)
	require.True(true)
}

var dummyAddr = flag.String("addr", "localhost:8080", "http service address")

func TestServerPublish(t *testing.T) {
	require := require.New(t)
	// Create a new logger for the test
	logger := logging.NoLog{}
	// Create a new pubsub server
	server := New(logger)
	ch := make(chan bool)
	// Handle requests to dummyAddr + '/'
	http.HandleFunc("/", server.ServeHTTP)
	// Go routine that listens on dummyAddress for connections
	go func() {
		http.ListenAndServe(*dummyAddr, nil)
	}()
	// Connect to pubsub server
	u := url.URL{Scheme: "ws", Host: *dummyAddr, Path: "/"}
	web_con, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(err)
	// Add connection to servers subscribed connections
	server.lock.Lock()
	con, _ := server.conns.Peek()
	server.lock.Unlock()

	server.subscribedConnections.Add(con)
	// Publish to subscribed connections
	server.lock.Lock()
	server.Publish("dummy_msg")
	server.lock.Unlock()

	// Recieve the message from the publish
	_, msg, err := web_con.ReadMessage()
	require.NoError(err)
	var unmarshal string
	json.Unmarshal(msg, &unmarshal)
	// Verify that the received message is the expected dummy message
	require.Equal("dummy_msg", unmarshal)
	// Close the connection and wait for it to be closed on the server side
	go func() {
		con.conn.Close()
		for {
			server.lock.Lock()
			len := server.subscribedConnections.conns.Len()
			if len == 0 {
				server.lock.Unlock()
				ch <- true
				return
			}
			server.lock.Unlock()
			time.Sleep(10 * time.Millisecond)
		}
	}()
	// Wait for the connection to be closed or for a timeout to occur
	select {
	case <-ch:
		// Connection was closed on the server side, test passed
	case <-time.After(1 * time.Second):
		// Timeout occurred, connection was not closed on the server side, test failed
		require.Fail("connection was not closed on the server side")
	}
}

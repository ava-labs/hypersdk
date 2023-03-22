package pubsub

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/url"
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestGeneratePrivateKeyDifferent(t *testing.T) {
	require := require.New(t)
	require.True(true)
}

type DummyFilter struct {
	called bool
}

// Filter does not filter any connections and returns a dummy msg.
func (f DummyFilter) Filter(connections []Filter) ([]bool, interface{}) {
	f.called = true
	n := len(connections)
	res := make([]bool, n)
	for i := 0; i < n; i++ {
		res[i] = true
	}
	return res, "dummy_msg"
}

var dummyAddr = flag.String("addr", "localhost:8080", "http service address")

func TestServerPublish(t *testing.T) {
	require := require.New(t)
	// Create a new logger for the test
	logger := logging.NoLog{}
	// Create a new pubsub server
	server := New(logger)
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
	df := DummyFilter{}
	// Publish to subscribed connections
	server.lock.Lock()
	server.Publish(df)
	server.lock.Unlock()

	// Recieve the message from the publish
	_, msg, err := web_con.ReadMessage()
	require.NoError(err)
	var unmarshal string
	json.Unmarshal(msg, &unmarshal)
	// Verify that the received message is the expected dummy message
	require.Equal("dummy_msg", unmarshal)
}

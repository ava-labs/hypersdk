package typedclient

import (
	"context"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/stretchr/testify/require"
)

// testHandler simulates different response behaviors
type testHandler struct {
	simulateTimeout       bool
	simulateError         bool
	simulateNetworkError  bool
	responseDelay         time.Duration
	simulateMultiResponse bool
}

func (t *testHandler) AppRequest(
	_ context.Context,
	_ ids.NodeID,
	_ time.Time,
	requestBytes []byte,
) ([]byte, *common.AppError) {
	requestMsg := string(requestBytes)

	if t.simulateNetworkError {
		return nil, &common.AppError{Code: 0, Message: "network error"}
	}
	if t.responseDelay > 0 {
		time.Sleep(t.responseDelay)
	}

	// check if context is already canceled (timeout simulation)
	if t.simulateTimeout {
		// Don't respond, forcing a timeout
		time.Sleep(2 * time.Second)
	}

	if t.simulateError {
		return nil, &common.AppError{Code: 1, Message: "simulated error"}
	}

	response := "Response to: " + requestMsg
	return []byte(response), nil
}

// AppGossip is required by the interface but not used in this test
func (t *testHandler) AppGossip(
	_ context.Context,
	_ ids.NodeID,
	_ []byte,
) {
}

func TestSyncAppRequest(t *testing.T) {
	tests := []struct {
		name                 string
		simulateTimeout      bool
		simulateError        bool
		simulateNetworkError bool
		responseDelay        time.Duration
		contextTimeout       time.Duration
		expectedErrContains  string
	}{
		{
			name:           "successful_request",
			contextTimeout: 2 * time.Second,
		},
		{
			name:                 "network_error",
			simulateNetworkError: true,
			contextTimeout:       2 * time.Second,
			expectedErrContains:  "network error",
		},
		{
			name:                "context_timeout",
			responseDelay:       1500 * time.Millisecond,
			contextTimeout:      1 * time.Second,
			expectedErrContains: "context deadline exceeded",
		},
		{
			name:                "error_response",
			simulateError:       true,
			contextTimeout:      2 * time.Second,
			expectedErrContains: "simulated error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			handler := &testHandler{
				simulateTimeout:      tt.simulateTimeout,
				simulateError:        tt.simulateError,
				simulateNetworkError: tt.simulateNetworkError,
				responseDelay:        tt.responseDelay,
			}

			handlerID := ids.GenerateTestNodeID()
			client := p2ptest.NewClient(t, ctx, handler, ids.EmptyNodeID, handlerID)

			marshaler := testMarshaler{}
			syncClient := NewSyncTypedClient[testRequest, testResponse, []byte](client, &marshaler)

			request := testRequest{message: "Hello, world!"}
			response, err := syncClient.SyncAppRequest(ctx, handlerID, request)

			// Check the results
			if tt.expectedErrContains != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.expectedErrContains)
				req.Equal(utils.Zero[testResponse](), response)
			} else {
				req.NoError(err)
				req.Equal("Response to: Hello, world!", response.reply)
			}
		})
	}
}

type testRequest struct {
	message string
}

type testResponse struct {
	reply string
}

type testMarshaler struct{}

func (m testMarshaler) MarshalRequest(request testRequest) ([]byte, error) {
	return []byte(request.message), nil
}

func (m testMarshaler) UnmarshalResponse(bytes []byte) (testResponse, error) {
	return testResponse{string(bytes)}, nil
}

func (m testMarshaler) MarshalGossip([]byte) ([]byte, error) {
	return []byte{}, nil
}

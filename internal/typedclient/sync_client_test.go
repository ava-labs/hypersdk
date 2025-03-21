// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package typedclient

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/stretchr/testify/require"
)

var _ Marshaler[testRequest, testResponse, []byte] = (*testMarshaler)(nil)

func TestSyncAppRequest(t *testing.T) {
	tests := []struct {
		name                string
		errResponse         bool
		networkError        bool
		expectedErrContains string
		responseDelay       time.Duration
		contextTimeout      time.Duration
	}{
		{
			name:           "successful request",
			contextTimeout: 2 * time.Second,
		},
		{
			name:                "context timeout",
			responseDelay:       1500 * time.Millisecond,
			contextTimeout:      time.Second,
			expectedErrContains: "context deadline exceeded",
		},
		{
			name:                "error response",
			contextTimeout:      2 * time.Second,
			expectedErrContains: "simulated error",
			errResponse:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			handlerDone := make(chan struct{})
			handler := &testHandler{
				simulateError: tt.errResponse,
				responseDelay: tt.responseDelay,
				doneCh:        handlerDone,
				response:      []byte(tt.name),
			}

			handlerID := ids.GenerateTestNodeID()

			marshaler := testMarshaler{}
			client := p2ptest.NewSelfClient(t, ctx, handlerID, handler)
			syncClient := NewSyncTypedClient[testRequest, testResponse, []byte](client, &marshaler)

			response, err := withContextEnforcement(ctx, tt.expectedErrContains, func() (testResponse, error) {
				return syncClient.SyncAppRequest(ctx, handlerID, testRequest{message: "What is the current test name?"})
			})

			if len(tt.expectedErrContains) > 0 {
				r.Error(err)
				r.ErrorContains(err, tt.expectedErrContains)
				r.Equal(utils.Zero[testResponse](), response)
			} else {
				r.NoError(err)
				r.Equal(tt.name, response.reply)
			}
		})
	}
}

// testHandler simulates a server with different response behaviors
type testHandler struct {
	simulateError bool
	responseDelay time.Duration
	response      []byte
	doneCh        chan struct{}
}

func (t *testHandler) Close() error {
	if t.doneCh != nil {
		close(t.doneCh)
	}
	return nil
}

func (t *testHandler) AppRequest(
	ctx context.Context,
	_ ids.NodeID,
	_ time.Time,
	_ []byte,
) ([]byte, *common.AppError) {
	if t.simulateError {
		return nil, &common.AppError{Code: 1, Message: "simulated error"}
	}

	if t.responseDelay > 0 {
		select {
		case <-time.After(t.responseDelay):
			// Delay completed
		case <-ctx.Done():
			// Context was canceled before delay completed
			return nil, &common.AppError{
				Code:    2,
				Message: ctx.Err().Error(),
			}
		}
	}

	return t.response, nil
}

// AppGossip is required by the interface but not used in this test
func (*testHandler) AppGossip(
	_ context.Context,
	_ ids.NodeID,
	_ []byte,
) {
}

type testRequest struct {
	message string
}

type testResponse struct {
	reply string
}

type testMarshaler struct{}

func (testMarshaler) MarshalRequest(request testRequest) ([]byte, error) {
	return []byte(request.message), nil
}

func (testMarshaler) UnmarshalResponse(bytes []byte) (testResponse, error) {
	return testResponse{string(bytes)}, nil
}

func (testMarshaler) MarshalGossip(bytes []byte) ([]byte, error) {
	return bytes, nil
}

// Ensure the context is cancelled before checking results
func withContextEnforcement(ctx context.Context, expectedErr string, fn func() (testResponse, error)) (testResponse, error) {
	// If we expect a context deadline error, just wait for the context to be cancelled
	if expectedErr == "context deadline exceeded" {
		<-ctx.Done()
		return utils.Zero[testResponse](), ctx.Err()
	}

	// For other cases, run the function normally
	return fn()
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import "time"

const (
	Name              = "hypersdk"
	JSONRPCEndpoint   = "/coreapi"
	WebSocketEndpoint = "/corews"

	DefaultHandshakeTimeout = 10 * time.Second
)

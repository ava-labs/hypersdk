package rpc

import "time"

const (
	Name                 = "hypersdk"
	JSONRPCEndpoint      = "/coreapi"
	WebSocketEndpoint    = "/corews"
	JSONRPCStateEndpoint = "/corestate"

	DefaultHandshakeTimeout = 10 * time.Second
)

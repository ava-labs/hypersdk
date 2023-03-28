// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/units"
)

const (
	// Size of the ws read buffer
	readBufferSize = units.KiB

	// Size of the ws write buffer
	writeBufferSize = units.KiB

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 10 * units.KiB // bytes

	// Maximum number of pending messages to send to a peer.
	maxPendingMessages = 1024 // messages
)

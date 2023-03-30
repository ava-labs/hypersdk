// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/units"
)

const (
	readBufferSize     = units.KiB
	writeBufferSize    = units.KiB
	writeWait          = 10 * time.Second
	pongWait           = 60 * time.Second
	pingPeriod         = (pongWait * 9) / 10
	maxMessageSize     = 10 * units.KiB // bytes
	maxPendingMessages = 1024
	readHeaderTimeout  = 5 * time.Second
)

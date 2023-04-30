// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/units"
)

const (
	ReadBufferSize     = units.KiB
	WriteBufferSize    = units.KiB
	WriteWait          = 10 * time.Second
	PongWait           = 60 * time.Second
	PingPeriod         = (PongWait * 9) / 10
	MaxMessageSize     = 10 * units.KiB // bytes
	MaxPendingMessages = 1024
)

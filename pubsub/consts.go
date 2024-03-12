// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"time"

	"github.com/ava-labs/hypersdk/consts"
)

const (
	ReadBufferSize      = consts.NetworkSizeLimit
	WriteBufferSize     = consts.NetworkSizeLimit
	WriteWait           = 10 * time.Second
	PongWait            = 60 * time.Second
	PingPeriod          = (PongWait * 9) / 10
	MaxReadMessageSize  = consts.NetworkSizeLimit
	MaxWriteMessageSize = consts.NetworkSizeLimit
	MaxMessageWait      = 50 * time.Millisecond
	MaxPendingMessages  = 1024
)

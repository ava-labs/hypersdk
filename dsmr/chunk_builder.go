// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"time"

	"github.com/ava-labs/avalanchego/ids"
)

type Tx interface {
	GetID() ids.ID
	GetExpiry() time.Time
}

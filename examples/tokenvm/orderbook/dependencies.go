// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package orderbook

import (
	"github.com/ava-labs/avalanchego/utils/logging"
)

type Controller interface {
	Logger() logging.Logger
}

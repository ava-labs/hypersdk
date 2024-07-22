// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import "github.com/ava-labs/avalanchego/ids"

type Description struct {
	NetworkID uint32
	ChainID   ids.ID
}

type Network interface {
	URIs() []string
	Description() Description
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
)

type AuthEngine interface {
	GetBatchVerifier(cores int, count int) chain.AuthBatchVerifier
	Cache(auth chain.Auth)
}

type TxRemovedEvent struct {
	TxID ids.ID
	Err  error
}

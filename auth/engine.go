// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import "github.com/ava-labs/hypersdk/chain"

type Engine interface {
	GetBatchVerifier(cores int, count int) chain.AuthBatchVerifier
	Cache(auth chain.Auth)
}

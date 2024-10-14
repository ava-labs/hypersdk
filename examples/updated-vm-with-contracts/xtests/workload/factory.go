// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
)

type factory struct {
	factories []*auth.ED25519Factory
	addrs     []codec.Address
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package common

import (
	"github.com/ava-labs/hypersdk/auth/ed25519"
	"github.com/ava-labs/hypersdk/vm"
)

func Engines() map[uint8]vm.AuthEngine {
	return map[uint8]vm.AuthEngine{
		ED25519ID: &ed25519.ED25519AuthEngine{},
	}
}

/*
 * Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codectest

import (
	"crypto/rand"

	"github.com/ava-labs/hypersdk/codec"
)

// NewRandomAddress returns a random address
// for use during testing
func NewRandomAddress() (codec.Address, error) {
	b := make([]byte, codec.AddressLen)
	if _, err := rand.Read(b); err != nil {
		return codec.EmptyAddress, err
	}
	return codec.ToAddress(b)
}

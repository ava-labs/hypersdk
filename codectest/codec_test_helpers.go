// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codectest

import (
	"crypto/rand"

	"github.com/ava-labs/hypersdk/codec"
)

// CreateRandomAddress returns a random address
// for use during testing
func CreateRandomAddress() (codec.Address, error) {
	var randAddress codec.Address
	b := make([]byte, codec.AddressLen)
	_, err := rand.Read(b)
	if err != nil {
		return codec.EmptyAddress, err
	}
	copy(randAddress[:], b)
	return randAddress, nil
}

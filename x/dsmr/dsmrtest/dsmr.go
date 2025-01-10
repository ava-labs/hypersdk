// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmrtest

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
)

type Tx struct {
	ID      ids.ID        `serialize:"true"`
	Expiry  int64         `serialize:"true"`
	Sponsor codec.Address `serialize:"true"`
}

func (t Tx) GetID() ids.ID {
	return t.ID
}

func (t Tx) GetExpiry() int64 {
	return t.Expiry
}

func (t Tx) GetSponsor() codec.Address {
	return t.Sponsor
}

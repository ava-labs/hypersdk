// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//go:generate go run github.com/StephenButtolph/canoto/canoto --concurrent=false $GOFILE

package dsmrtest

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
)

type Tx struct {
	ID      ids.ID        `canoto:"fixed bytes,1"`
	Expiry  int64         `canoto:"int,2"`
	Sponsor codec.Address `canoto:"fixed bytes,3"`

	canotoData canotoData_Tx
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

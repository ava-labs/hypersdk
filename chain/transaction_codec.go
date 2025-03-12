// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

import (
	"github.com/ava-labs/hypersdk/codec"
)

// SerializeTx defines the canoto serialization format to serialize a transaction.
// If a byte slice field is empty, then it's omitted from the serialization. This means
// we can re-use [SerializeTx] to serialize both unsigned and signed transactions.
// To serialize an unsigned tx, we omit the [Auth] field. To parse, we verify the [Auth]
// field is empty one level above.
type SerializeTx struct {
	Base    Base          `canoto:"value,1"`
	Actions []codec.Bytes `canoto:"repeated bytes,2"`
	Auth    codec.Bytes   `canoto:"bytes,3"`

	canotoData canotoData_SerializeTx
}

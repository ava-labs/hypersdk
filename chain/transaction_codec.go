// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/hypersdk/codec"
)

type SerializeTxData struct {
	Base    *Base         `canoto:"pointer,1"`
	Actions []codec.Bytes `canoto:"repeated bytes,2"`

	canotoData canotoData_SerializeTxData
}

type SerializeRawTx struct {
	TransactionData []byte      `canoto:"bytes,1"`
	Auth            codec.Bytes `canoto:"bytes,2"`

	canotoData canotoData_SerializeRawTx
}

type SerializeTx struct {
	TransactionData `canoto:"value,1"`
	Auth            codec.Bytes `canoto:"bytes,2"`

	canotoData canotoData_SerializeTx
}

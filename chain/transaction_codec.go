// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/hypersdk/codec"
)

// SerializeTxData defines the canoto serialization format of TransactionData
// containing a Base and raw bytes for each action included in the transaction.
type SerializeTxData struct {
	Base    Base          `canoto:"value,1"`
	Actions []codec.Bytes `canoto:"repeated bytes,2"`

	canotoData canotoData_SerializeTxData
}

// SerializeRawTxData defines the canoto serialization format used for serializing
// a transaction's raw bytes with an added auth field.
// The serialization format is identical using [SerializeRawTxData] or [SerializeTx]
// with the TransactionData type replacing the TransactionData raw bytes field.
type SerializeRawTxData struct {
	TransactionData []byte      `canoto:"bytes,1"`
	Auth            codec.Bytes `canoto:"bytes,2"`

	canotoData canotoData_SerializeRawTxData
}

// SerializeTx defines the canoto serialization format used for serializing a transaction.
// To deserialize TransactionData correctly, we assume that Canoto context has been populated
// with a [TxParser] instance.
type SerializeTx struct {
	// This field is serialized identically by canoto when using a value or
	// treating this field as raw bytes as in [SerializeRawTxData]
	TransactionData `canoto:"value,1"`
	Auth            codec.Bytes `canoto:"bytes,2"`

	canotoData canotoData_SerializeTx
}

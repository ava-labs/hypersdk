// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/StephenButtolph/canoto"

	"github.com/ava-labs/hypersdk/codec"
)

var _ Parser = (*TxTypeParser)(nil)

type TxTypeParser struct {
	ActionRegistry *codec.TypeParser[Action]
	AuthRegistry   *codec.TypeParser[Auth]
}

func NewTxTypeParser(
	actionRegistry *codec.TypeParser[Action],
	authRegistry *codec.TypeParser[Auth],
) *TxTypeParser {
	return &TxTypeParser{
		ActionRegistry: actionRegistry,
		AuthRegistry:   authRegistry,
	}
}

func (t *TxTypeParser) ParseAction(bytes []byte) (Action, error) {
	return t.ActionRegistry.Unmarshal(bytes)
}

func (t *TxTypeParser) ParseAuth(bytes []byte) (Auth, error) {
	return t.AuthRegistry.Unmarshal(bytes)
}

type BatchedTransactions struct {
	Transactions []*Transaction `canoto:"repeated pointer,1"`

	canotoData canotoData_BatchedTransactions
}

type BatchedTransactionSerializer struct {
	Parser Parser
}

func (b *BatchedTransactionSerializer) Marshal(txs []*Transaction) []byte {
	batch := BatchedTransactions{Transactions: txs}
	return batch.MarshalCanoto()
}

func (b *BatchedTransactionSerializer) Unmarshal(bytes []byte) ([]*Transaction, error) {
	reader := canoto.Reader{
		B:       bytes,
		Context: b.Parser,
	}
	batch := &BatchedTransactions{}
	if err := batch.UnmarshalCanotoFrom(reader); err != nil {
		return nil, err
	}
	return batch.Transactions, nil
}

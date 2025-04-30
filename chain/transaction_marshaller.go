// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"

	"github.com/StephenButtolph/canoto"
)

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

var _ Parser = (*TxTypeParser)(nil)

type TypeParser[T any] interface {
	Unmarshal([]byte) (T, error)
}

type TxTypeParser struct {
	ActionParser TypeParser[Action]
	AuthParser   TypeParser[Auth]
}

func NewTxTypeParser(
	actionParser TypeParser[Action],
	authParser TypeParser[Auth],
) *TxTypeParser {
	return &TxTypeParser{
		ActionParser: actionParser,
		AuthParser:   authParser,
	}
}

func (t *TxTypeParser) ParseAction(bytes []byte) (Action, error) {
	return t.ActionParser.Unmarshal(bytes)
}

func (t *TxTypeParser) ParseAuth(bytes []byte) (Auth, error) {
	return t.AuthParser.Unmarshal(bytes)
}

type BatchedTransactions struct {
	Transactions []*Transaction `canoto:"repeated pointer,1"`

	canotoData canotoData_BatchedTransactions
}

type BatchedTransactionSerializer struct {
	Parser Parser
}

func (*BatchedTransactionSerializer) Marshal(txs []*Transaction) []byte {
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
	for i, tx := range batch.Transactions {
		if tx == nil {
			return nil, fmt.Errorf("%w: at index %d", ErrNilTxInBlock, i)
		}
	}
	return batch.Transactions, nil
}

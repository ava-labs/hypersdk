// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

type TxSerializer[T Action[T], A Auth[A]] struct{}

type Transactions[T Action[T], A Auth[A]] struct {
	Txs []*Transaction[T, A] `canoto:"repeated field,1"`

	canotoData canotoData_Transactions
}

func (*TxSerializer[T, A]) Unmarshal(data []byte) ([]*Transaction[T, A], error) {
	txMsg := &Transactions[T, A]{}
	if err := txMsg.UnmarshalCanoto(data); err != nil {
		return nil, err
	}
	return txMsg.Txs, nil
}

func (*TxSerializer[T, A]) Marshal(txs []*Transaction[T, A]) ([]byte, error) {
	if len(txs) == 0 {
		return nil, ErrNoTxs
	}
	txMsg := &Transactions[T, A]{Txs: txs}
	return txMsg.MarshalCanoto(), nil
}

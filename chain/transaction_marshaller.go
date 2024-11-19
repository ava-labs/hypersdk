// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

// initialCapacity is the initial size of a txs array we allocate when
// unmarshaling a batch of txs.
const initialCapacity = 1000

type TxSerializer struct {
	ActionRegistry *codec.TypeParser[Action]
	AuthRegistry   *codec.TypeParser[Auth]
}

func (s *TxSerializer) Unmarshal(data []byte) ([]*Transaction, error) {
	p := codec.NewReader(data, consts.NetworkSizeLimit)
	txCount := p.UnpackInt(true)
	txs := make([]*Transaction, 0, min(txCount, initialCapacity)) // DoS to set size to txCount
	for i := uint32(0); i < txCount; i++ {
		tx, err := UnmarshalTx(p, s.ActionRegistry, s.AuthRegistry)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	if !p.Empty() {
		// Ensure no leftover bytes
		return nil, ErrInvalidObject
	}
	return txs, p.Err()
}

func (*TxSerializer) Marshal(txs []*Transaction) ([]byte, error) {
	if len(txs) == 0 {
		return nil, ErrNoTxs
	}
	size := consts.IntLen + codec.CummSize(txs)
	p := codec.NewWriter(size, consts.NetworkSizeLimit)
	p.PackInt(uint32(len(txs)))
	for _, tx := range txs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
	}
	return p.Bytes(), p.Err()
}

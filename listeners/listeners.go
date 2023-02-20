// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package listeners

import (
	"sync"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/emap"
)

type (
	TxListener    chan *Transaction
	BlockListener chan *chain.StatelessBlock
)

type Transaction struct {
	TxID   ids.ID
	Result *chain.Result
	Err    error
}

type Listeners struct {
	txL         sync.Mutex
	txListeners map[ids.ID][]TxListener
	expiringTxs *emap.EMap[*chain.Transaction] // ensures all tx listeners are eventually responded to

	blockL         sync.Mutex
	blockListeners map[ids.ID]BlockListener // ids.ID is random connection identifier
}

func New() *Listeners {
	return &Listeners{
		txListeners: map[ids.ID][]TxListener{},
		expiringTxs: emap.NewEMap[*chain.Transaction](),

		blockListeners: map[ids.ID]BlockListener{},
	}
}

// Note: no need to have a tx listener removal, this will happen when all
// submitted transactions are cleared.
func (w *Listeners) AddTxListener(tx *chain.Transaction, c TxListener) {
	w.txL.Lock()
	defer w.txL.Unlock()

	txID := tx.ID()
	w.txListeners[txID] = append(w.txListeners[txID], c)
	w.expiringTxs.Add([]*chain.Transaction{tx})
}

func (w *Listeners) AddBlockListener(id ids.ID, c BlockListener) {
	w.blockL.Lock()
	defer w.blockL.Unlock()

	w.blockListeners[id] = c
}

func (w *Listeners) RemoveBlockListener(id ids.ID) {
	w.blockL.Lock()
	defer w.blockL.Unlock()

	delete(w.blockListeners, id)
}

// If never possible for a tx to enter mempool, call this
func (w *Listeners) RemoveTx(txID ids.ID, err error) {
	w.txL.Lock()
	defer w.txL.Unlock()

	w.removeTx(txID, err)
}

func (w *Listeners) removeTx(txID ids.ID, err error) {
	listeners, ok := w.txListeners[txID]
	if !ok {
		return
	}
	for _, listener := range listeners {
		select {
		case listener <- &Transaction{
			TxID: txID,
			Err:  err,
		}:
		default:
			// drop message if client is not keeping up or abandoned
		}
	}
	delete(w.txListeners, txID)
	// [expiringTxs] will be cleared eventually (does not support removal)
}

func (w *Listeners) SetMinTx(t int64) {
	w.txL.Lock()
	defer w.txL.Unlock()

	expired := w.expiringTxs.SetMin(t)
	for _, id := range expired {
		w.removeTx(id, ErrExpired)
	}
}

func (w *Listeners) AcceptBlock(b *chain.StatelessBlock) {
	w.blockL.Lock()
	for _, listener := range w.blockListeners {
		select {
		case listener <- b:
		default:
			// drop message if client is not keeping up or abandoned
		}
	}
	w.blockL.Unlock()

	w.txL.Lock()
	defer w.txL.Unlock()
	results := b.Results()
	for i, tx := range b.Txs {
		txID := tx.ID()
		listeners, ok := w.txListeners[txID]
		if !ok {
			return
		}
		for _, listener := range listeners {
			select {
			case listener <- &Transaction{
				TxID:   txID,
				Result: results[i],
			}:
			default:
				// drop message if client is not keeping up or abandoned
			}
		}
		delete(w.txListeners, txID)
		// [expiringTxs] will be cleared eventually (does not support removal)
	}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"encoding/binary"
	"errors"
	"path/filepath"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/vm"
)

var (
	failureByte = byte(0x0)
	successByte = byte(0x1)

	_ event.SubscriptionFactory[*chain.StatelessBlock] = (*subscriptionFactory)(nil)
	_ event.Subscription[*chain.StatelessBlock]        = (*txDBIndexer)(nil)
)

func WithIndexer(name string, path string) vm.Option {
	return func(v *vm.VM) error {
		dbPath := filepath.Join(v.DataDir, "indexer", "db")
		db, _, err := pebble.New(dbPath, pebble.NewDefaultConfig())
		if err != nil {
			return err
		}

		indexer := &txDBIndexer{
			db: db,
		}

		subscriptionFactory := &subscriptionFactory{
			indexer: indexer,
		}

		apiFactory := &apiFactory{
			path:    path,
			name:    name,
			indexer: indexer,
		}

		if err := vm.WithBlockSubscriptions(subscriptionFactory)(v); err != nil {
			return err
		}

		return vm.WithVMAPIs(apiFactory)(v)
	}
}

type subscriptionFactory struct {
	indexer *txDBIndexer
}

func (s *subscriptionFactory) New() (event.Subscription[*chain.StatelessBlock], error) {
	return s.indexer, nil
}

type txDBIndexer struct {
	db database.Database
}

func (t *txDBIndexer) Accept(blk *chain.StatelessBlock) error {
	batch := t.db.NewBatch()
	defer batch.Reset()

	timestamp := blk.GetTimestamp()
	results := blk.Results()
	for j, tx := range blk.Txs {
		result := results[j]
		if err := t.storeTransaction(
			batch,
			tx.ID(),
			timestamp,
			result.Success,
			result.Units,
			result.Fee,
		); err != nil {
			return err
		}
	}

	return batch.Write()
}

func (t *txDBIndexer) Close() error {
	return t.db.Close()
}

func (*txDBIndexer) storeTransaction(
	batch database.KeyValueWriter,
	txID ids.ID,
	timestamp int64,
	success bool,
	units fees.Dimensions,
	fee uint64,
) error {
	v := make([]byte, consts.Uint64Len+1+fees.DimensionsLen+consts.Uint64Len)
	binary.BigEndian.PutUint64(v, uint64(timestamp))
	if success {
		v[consts.Uint64Len] = successByte
	} else {
		v[consts.Uint64Len] = failureByte
	}
	copy(v[consts.Uint64Len+1:], units.Bytes())
	binary.BigEndian.PutUint64(v[consts.Uint64Len+1+fees.DimensionsLen:], fee)
	return batch.Put(txID[:], v)
}

func (t *txDBIndexer) GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
	v, err := t.db.Get(txID[:])
	if errors.Is(err, database.ErrNotFound) {
		return false, 0, false, fees.Dimensions{}, 0, nil
	}
	if err != nil {
		return false, 0, false, fees.Dimensions{}, 0, err
	}
	timestamp := int64(binary.BigEndian.Uint64(v))
	success := true
	if v[consts.Uint64Len] == failureByte {
		success = false
	}
	d, err := fees.UnpackDimensions(v[consts.Uint64Len+1 : consts.Uint64Len+1+fees.DimensionsLen])
	if err != nil {
		return false, 0, false, fees.Dimensions{}, 0, err
	}
	fee := binary.BigEndian.Uint64(v[consts.Uint64Len+1+fees.DimensionsLen:])
	return true, timestamp, success, d, fee, nil
}

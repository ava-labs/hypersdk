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
	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/ava-labs/hypersdk/vm"
)

const (
	Name      = "indexer"
	Namespace = "indexer"
)

var (
	failureByte = byte(0x0)
	successByte = byte(0x1)

	_ event.SubscriptionFactory[*chain.StatefulBlock[chain.RuntimeInterface]] = (*subscriptionFactory[chain.RuntimeInterface])(nil)
	_ event.Subscription[*chain.StatefulBlock[chain.RuntimeInterface]]        = (*txDBIndexer[chain.RuntimeInterface])(nil)
)

type Config struct {
	Enabled bool `json:"enabled"`
}

func NewDefaultConfig() Config {
	return Config{
		Enabled: true,
	}
}

func With[T chain.RuntimeInterface]() vm.Option[T] {
	return vm.NewOption[T](Namespace, NewDefaultConfig(), OptionFunc[T])
}

func OptionFunc[T chain.RuntimeInterface](v *vm.VM[T], config Config) error {
	if !config.Enabled {
		return nil
	}
	dbPath := filepath.Join(v.DataDir, "indexer", "db")
	db, _, err := pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		return err
	}

	indexer := &txDBIndexer[T]{
		db: db,
	}

	subscriptionFactory := &subscriptionFactory[T]{
		indexer: indexer,
	}

	apiFactory := &apiFactory[T]{
		path:    Endpoint,
		name:    Name,
		indexer: indexer,
	}

	vm.WithBlockSubscriptions[T](subscriptionFactory)(v)
	vm.WithVMAPIs[T](apiFactory)(v)

	return nil
}

type subscriptionFactory[T chain.RuntimeInterface] struct {
	indexer *txDBIndexer[T]
}

func (s *subscriptionFactory[T]) New() (event.Subscription[*chain.StatefulBlock[T]], error) {
	return s.indexer, nil
}

type txDBIndexer[T chain.RuntimeInterface] struct {
	db database.Database
}

func (t *txDBIndexer[T]) Accept(blk *chain.StatefulBlock[T]) error {
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

func (t *txDBIndexer[_]) Close() error {
	return t.db.Close()
}

func (*txDBIndexer[_]) storeTransaction(
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

func (t *txDBIndexer[_]) GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
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

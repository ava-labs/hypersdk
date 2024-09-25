// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txindexer

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
	Name      = "txindexer"
	Namespace = "txindexer"
)

var (
	failureByte = byte(0x0)
	successByte = byte(0x1)

	_ event.SubscriptionFactory[*chain.StatefulBlock] = (*subscriptionFactory)(nil)
	_ event.Subscription[*chain.StatefulBlock]        = (*indexer)(nil)
)

type Config struct {
	Enabled bool `json:"enabled"`
}

func NewDefaultConfig() Config {
	return Config{
		Enabled: true,
	}
}

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), OptionFunc)
}

func OptionFunc(v *vm.VM, config Config) error {
	if !config.Enabled {
		return nil
	}
	dbPath := filepath.Join(v.DataDir, Namespace, "db")
	db, _, err := pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		return err
	}

	indexer := &indexer{db: db}

	subscriptionFactory := &subscriptionFactory{
		indexer: indexer,
	}

	apiFactory := &apiFactory{
		path:    Endpoint,
		name:    Name,
		indexer: indexer,
	}

	vm.WithBlockSubscriptions(subscriptionFactory)(v)
	vm.WithVMAPIs(apiFactory)(v)

	return nil
}

type subscriptionFactory struct {
	indexer *indexer
}

func (s *subscriptionFactory) New() (event.Subscription[*chain.StatefulBlock], error) {
	return s.indexer, nil
}

type indexer struct {
	db database.Database
}

func (i *indexer) Accept(blk *chain.StatefulBlock) error {
	batch := i.db.NewBatch()
	defer batch.Reset()

	timestamp := blk.GetTimestamp()
	results := blk.Results()
	for j, tx := range blk.Txs {
		result := results[j]
		if err := i.storeTransaction(
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

func (*indexer) storeTransaction(
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

func (i *indexer) GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, error) {
	v, err := i.db.Get(txID[:])
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

func (i *indexer) Close() error {
	return i.db.Close()
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"errors"
	"path/filepath"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
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
	_ event.SubscriptionFactory[*chain.ExecutedBlock] = (*subscriptionFactory)(nil)
	_ event.Subscription[*chain.ExecutedBlock]        = (*indexer)(nil)
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
	dbPath := filepath.Join(v.DataDir, Namespace)
	db, _, err := pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		return err
	}

	indexer := &indexer{
		db: db,
	}

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

func (s *subscriptionFactory) New() (event.Subscription[*chain.ExecutedBlock], error) {
	return s.indexer, nil
}

type indexer struct {
	db database.Database
}

func (t *indexer) Accept(blk *chain.ExecutedBlock) error {
	batch := t.db.NewBatch()
	defer batch.Reset()

	for j, tx := range blk.Txs {
		result := blk.Results[j]
		if err := t.storeTransaction(
			batch,
			tx.ID(),
			blk.Tmstmp,
			result.Success,
			result.Units,
			result.Fee,
			result.Outputs,
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
	outputs [][]byte,
) error {
	outputLength := consts.ByteLen // Single byte containing number of outputs
	for _, output := range outputs {
		outputLength += consts.Uint32Len + len(output)
	}
	txResultLength := consts.Uint64Len + consts.BoolLen + fees.DimensionsLen + consts.Uint64Len + outputLength

	writer := codec.NewWriter(txResultLength, txResultLength)
	writer.PackUint64(uint64(timestamp))
	writer.PackBool(success)
	writer.PackFixedBytes(units.Bytes())
	writer.PackUint64(fee)
	writer.PackByte(byte(len(outputs)))
	for _, output := range outputs {
		writer.PackBytes(output)
	}
	if err := writer.Err(); err != nil {
		return err
	}
	return batch.Put(txID[:], writer.Bytes())
}

func (t *indexer) GetTransaction(txID ids.ID) (bool, int64, bool, fees.Dimensions, uint64, [][]byte, error) {
	v, err := t.db.Get(txID[:])
	if errors.Is(err, database.ErrNotFound) {
		return false, 0, false, fees.Dimensions{}, 0, nil, nil
	}
	if err != nil {
		return false, 0, false, fees.Dimensions{}, 0, nil, err
	}
	reader := codec.NewReader(v, consts.NetworkSizeLimit)
	timestamp := reader.UnpackUint64(true)
	success := reader.UnpackBool()
	dimensionsBytes := make([]byte, fees.DimensionsLen)
	reader.UnpackFixedBytes(fees.DimensionsLen, &dimensionsBytes)
	fee := reader.UnpackUint64(true)
	numOutputs := int(reader.UnpackByte())
	outputs := make([][]byte, numOutputs)
	for i := range outputs {
		outputs[i] = reader.UnpackLimitedBytes(consts.NetworkSizeLimit)
	}
	if err := reader.Err(); err != nil {
		return false, 0, false, fees.Dimensions{}, 0, nil, err
	}
	dimensions, err := fees.UnpackDimensions(dimensionsBytes)
	if err != nil {
		return false, 0, false, fees.Dimensions{}, 0, nil, err
	}
	return true, int64(timestamp), success, dimensions, fee, outputs, nil
}

func (t *indexer) Close() error {
	return t.db.Close()
}

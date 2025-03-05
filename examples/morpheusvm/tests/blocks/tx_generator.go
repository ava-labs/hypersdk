// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"encoding/binary"
	"math/rand"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	_ chaintest.TxListGenerator = (*ParallelTxListGenerator)(nil)
	_ chaintest.TxListGenerator = (*SerialTxListGenerator)(nil)
	_ chaintest.TxListGenerator = (*ZipfTxListGenerator)(nil)
)

type ParallelTxListGenerator struct {
	nonce uint64
}

// Generate creates a list of transactions where each transaction sends 1 unit
// to itself.
func (p *ParallelTxListGenerator) Generate(rules chain.Rules, timestamp int64, unitPrices fees.Dimensions, factories []chain.AuthFactory, numOfTxs uint64) ([]*chain.Transaction, error) {
	txs := make([]*chain.Transaction, numOfTxs)
	for i := 0; i < int(numOfTxs); i++ {
		action := &actions.Transfer{
			To:    factories[i].Address(),
			Value: 1,
			Memo:  binary.BigEndian.AppendUint64(nil, p.nonce),
		}

		p.nonce++

		units, err := chain.EstimateUnits(rules, []chain.Action{action}, factories[i])
		if err != nil {
			return nil, err
		}

		maxFee, err := fees.MulSum(unitPrices, units)
		if err != nil {
			return nil, err
		}

		txData := chain.NewTxData(
			chain.Base{
				Timestamp: utils.UnixRMilli(timestamp, rules.GetValidityWindow()),
				ChainID:   rules.GetChainID(),
				MaxFee:    maxFee,
			},
			[]chain.Action{action},
		)

		tx, err := txData.Sign(factories[i])
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}

	return txs, nil
}

type SerialTxListGenerator struct {
	nonce uint64
}

// Generate creates a list of transaction where each transaction sends 1 unit to
// the zero address.
func (s *SerialTxListGenerator) Generate(rules chain.Rules, timestamp int64, unitPrices fees.Dimensions, factories []chain.AuthFactory, numOfTxs uint64) ([]*chain.Transaction, error) {
	txs := make([]*chain.Transaction, numOfTxs)
	for i := 0; i < int(numOfTxs); i++ {
		action := &actions.Transfer{
			To:    codec.EmptyAddress,
			Value: 1,
			Memo:  binary.BigEndian.AppendUint64(nil, s.nonce),
		}

		s.nonce++

		units, err := chain.EstimateUnits(rules, []chain.Action{action}, factories[i])
		if err != nil {
			return nil, err
		}

		maxFee, err := fees.MulSum(unitPrices, units)
		if err != nil {
			return nil, err
		}

		txData := chain.NewTxData(
			chain.Base{
				Timestamp: utils.UnixRMilli(timestamp, rules.GetValidityWindow()),
				ChainID:   rules.GetChainID(),
				MaxFee:    maxFee,
			},
			[]chain.Action{action},
		)

		tx, err := txData.Sign(factories[i])
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}

	return txs, nil
}

type ZipfTxListGenerator struct {
	nonce uint64
}

// Generate creates a list of transactions where each transaction sends 1 unit
// to X.
//
// X is sampled from a Zipf distribution where the sample set is a set of random addresses.
func (z *ZipfTxListGenerator) Generate(rules chain.Rules, timestamp int64, unitPrices fees.Dimensions, factories []chain.AuthFactory, numOfTxs uint64) ([]*chain.Transaction, error) {
	var (
		zipfSeed = rand.New(rand.NewSource(0))
		sZipf    = 1.01
		vZipf    = 2.7
	)
	zipfGen := rand.NewZipf(zipfSeed, sZipf, vZipf, numOfTxs-1)
	sampleAddresses := make([]codec.Address, numOfTxs)
	for i := 0; i < int(numOfTxs); i++ {
		sampleAddresses[i] = codectest.NewRandomAddress()
	}

	txs := make([]*chain.Transaction, numOfTxs)
	for i := 0; i < int(numOfTxs); i++ {
		to := sampleAddresses[zipfGen.Uint64()]

		action := &actions.Transfer{
			To:    to,
			Value: 1,
			Memo:  binary.BigEndian.AppendUint64(nil, z.nonce),
		}

		z.nonce++

		units, err := chain.EstimateUnits(rules, []chain.Action{action}, factories[i])
		if err != nil {
			return nil, err
		}

		maxFee, err := fees.MulSum(unitPrices, units)
		if err != nil {
			return nil, err
		}

		txData := chain.NewTxData(
			chain.Base{
				Timestamp: utils.UnixRMilli(timestamp, rules.GetValidityWindow()),
				ChainID:   rules.GetChainID(),
				MaxFee:    maxFee,
			},
			[]chain.Action{action},
		)

		tx, err := txData.Sign(factories[i])
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}
	return txs, nil
}

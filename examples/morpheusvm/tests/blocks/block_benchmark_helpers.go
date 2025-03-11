// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"encoding/binary"
	"math/rand"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/genesis"
)

func parallelTxsBlockBenchmarkHelper(numOfTxsPerBlock uint64) (genesis.Genesis, chaintest.TxListGenerator, error) {
	factories, gen, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, nil, err
	}

	nonce := uint64(0)

	txListGenerator := func(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
		txs := make([]*chain.Transaction, numOfTxsPerBlock)
		for i := 0; i < int(numOfTxsPerBlock); i++ {
			action := &actions.Transfer{
				To:    factories[i].Address(),
				Value: 1,
				Memo:  binary.BigEndian.AppendUint64(nil, nonce),
			}

			nonce++

			tx, err := txGenerator([]chain.Action{action}, factories[i])
			if err != nil {
				return nil, err
			}

			txs[i] = tx
		}

		return txs, nil
	}
	return gen, txListGenerator, nil
}

func serialTxsBlockBenchmarkHelper(numOfTxsPerBlock uint64) (genesis.Genesis, chaintest.TxListGenerator, error) {
	factories, gen, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, nil, err
	}

	nonce := uint64(0)

	txListGenerator := func(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
		txs := make([]*chain.Transaction, numOfTxsPerBlock)
		for i := 0; i < int(numOfTxsPerBlock); i++ {
			action := &actions.Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
				Memo:  binary.BigEndian.AppendUint64(nil, nonce),
			}

			tx, err := txGenerator([]chain.Action{action}, factories[i])
			if err != nil {
				return nil, err
			}

			txs[i] = tx
		}

		return txs, nil
	}
	return gen, txListGenerator, nil
}

func zipfTxsBlockBenchmarkHelper(numOfTxsPerBlock uint64) (genesis.Genesis, chaintest.TxListGenerator, error) {
	factories, gen, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, nil, err
	}

	nonce := uint64(0)

	zipfSeed := rand.New(rand.NewSource(0)) //nolint:gosec
	sZipf := 1.01
	vZipf := 2.7
	zipfGen := rand.NewZipf(zipfSeed, sZipf, vZipf, numOfTxsPerBlock-1)

	txListGenerator := func(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
		txs := make([]*chain.Transaction, numOfTxsPerBlock)
		for i := 0; i < int(numOfTxsPerBlock); i++ {
			action := &actions.Transfer{
				To:    factories[zipfGen.Uint64()].Address(),
				Value: 1,
				Memo:  binary.BigEndian.AppendUint64(nil, nonce),
			}

			tx, err := txGenerator([]chain.Action{action}, factories[i])
			if err != nil {
				return nil, err
			}

			txs[i] = tx
		}

		return txs, nil
	}
	return gen, txListGenerator, nil
}

func createGenesis(numOfFactories uint64, allocAmount uint64) ([]chain.AuthFactory, genesis.Genesis, error) {
	factories := make([]chain.AuthFactory, numOfFactories)
	customAllocs := make([]*genesis.CustomAllocation, numOfFactories)
	for i := range numOfFactories {
		pk, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return nil, nil, err
		}
		factory := auth.NewED25519Factory(pk)
		factories[i] = factory
		customAllocs[i] = &genesis.CustomAllocation{
			Address: factory.Address(),
			Balance: allocAmount,
		}
	}
	return factories, genesis.NewDefaultGenesis(customAllocs), nil
}

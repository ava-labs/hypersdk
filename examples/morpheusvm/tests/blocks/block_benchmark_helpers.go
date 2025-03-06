// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"encoding/binary"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/genesis"
)

var (
	_ chaintest.BlockBenchmarkHelper = (*ParallelTxBlockBenchmarkHelper)(nil)
	_ chaintest.BlockBenchmarkHelper = (*SerialTxBlockBenchmarkHelper)(nil)
)

type ParallelTxBlockBenchmarkHelper struct {
	factories []chain.AuthFactory
	nonce     uint64
}

func (p *ParallelTxBlockBenchmarkHelper) GenerateGenesis(numOfTxsPerBlock uint64) (genesis.Genesis, error) {
	factories, gen, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, err
	}
	p.factories = factories
	return gen, nil
}

func (p *ParallelTxBlockBenchmarkHelper) GenerateTxList(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
	txs := make([]*chain.Transaction, numOfTxsPerBlock)
	for i := 0; i < int(numOfTxsPerBlock); i++ {
		action := &actions.Transfer{
			To:    p.factories[i].Address(),
			Value: 1,
			Memo:  binary.BigEndian.AppendUint64(nil, p.nonce),
		}

		p.nonce++

		tx, err := txGenerator([]chain.Action{action}, p.factories[i])
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}

	return txs, nil
}

type SerialTxBlockBenchmarkHelper struct {
	factories []chain.AuthFactory
	nonce     uint64
}

func (s *SerialTxBlockBenchmarkHelper) GenerateGenesis(numOfTxsPerBlock uint64) (genesis.Genesis, error) {
	factories, gen, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, err
	}
	s.factories = factories
	return gen, nil
}

func (s *SerialTxBlockBenchmarkHelper) GenerateTxList(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
	txs := make([]*chain.Transaction, numOfTxsPerBlock)
	for i := 0; i < int(numOfTxsPerBlock); i++ {
		action := &actions.Transfer{
			To:    s.factories[i].Address(),
			Value: 1,
			Memo:  binary.BigEndian.AppendUint64(nil, s.nonce),
		}

		s.nonce++

		tx, err := txGenerator([]chain.Action{action}, s.factories[i])
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}

	return txs, nil
}

type ZipfTxBlockBenchmarkHelper struct {
	factories []chain.AuthFactory
	nonce     uint64
}

func (z *ZipfTxBlockBenchmarkHelper) GenerateGenesis(numOfTxsPerBlock uint64) (genesis.Genesis, error) {
	factories, gen, err := createGenesis(numOfTxsPerBlock, 1_000_000)
	if err != nil {
		return nil, err
	}
	z.factories = factories
	return gen, nil
}

func (z *ZipfTxBlockBenchmarkHelper) GenerateTxList(numOfTxsPerBlock uint64, txGenerator chaintest.TxGenerator) ([]*chain.Transaction, error) {
	txs := make([]*chain.Transaction, numOfTxsPerBlock)
	for i := 0; i < int(numOfTxsPerBlock); i++ {
		action := &actions.Transfer{
			To:    z.factories[i].Address(),
			Value: 1,
			Memo:  binary.BigEndian.AppendUint64(nil, z.nonce),
		}

		z.nonce++

		tx, err := txGenerator([]chain.Action{action}, z.factories[i])
		if err != nil {
			return nil, err
		}

		txs[i] = tx
	}

	return txs, nil
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

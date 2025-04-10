// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/hyperevm/actions"
	"github.com/ava-labs/hypersdk/examples/hyperevm/consts"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/load"
	"github.com/ava-labs/hypersdk/state"
)

var (
	ErrTxGeneratorFundsExhausted = errors.New("tx generator funds exhausted")

	_ load.TxGenerator[*chain.Transaction] = (*TxGenerator)(nil)
)

type TxGenerator struct {
	authFactory chain.AuthFactory
	currBalance uint64
	ruleFactory chain.RuleFactory
	unitPrices  fees.Dimensions
	sent        uint64
}

func NewTxGenerator(
	authFactory chain.AuthFactory,
	ruleFactory chain.RuleFactory,
	currBalance uint64,
	unitPrices fees.Dimensions,
) *TxGenerator {
	return &TxGenerator{
		authFactory: authFactory,
		ruleFactory: ruleFactory,
		currBalance: currBalance,
		unitPrices:  unitPrices,
	}
}

func (t *TxGenerator) GenerateTx(context.Context) (*chain.Transaction, error) {
	memo := binary.BigEndian.AppendUint64(nil, t.sent)
	gas, err := core.IntrinsicGas(memo, nil, false, consts.DefaultRules)
	if err != nil {
		return nil, err
	}

	to := storage.ToEVMAddress(t.authFactory.Address())
	stateKeys := state.Keys{
		string(storage.AccountKey(to)): state.All,
		// Coinbase address
		string(storage.AccountKey(common.Address{})): state.All,
	}

	action := &actions.EvmCall{
		To:       to,
		From:     to,
		Value:    1,
		GasLimit: gas,
		Data:     memo,
		Keys:     stateKeys,
	}

	tx, err := chain.GenerateTransaction(
		t.ruleFactory,
		t.unitPrices,
		time.Now().UnixMilli(),
		[]chain.Action{action},
		t.authFactory,
	)
	if err != nil {
		return nil, err
	}
	if tx.MaxFee() > t.currBalance {
		return nil, ErrTxGeneratorFundsExhausted
	}
	t.currBalance -= tx.MaxFee()
	t.sent++
	return tx, nil
}

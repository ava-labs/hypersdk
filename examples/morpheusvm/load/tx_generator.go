// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/load"
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
	tx, err := chain.GenerateTransaction(
		t.ruleFactory,
		t.unitPrices,
		time.Now().UnixMilli(),
		[]chain.Action{
			&actions.Transfer{
				To:    t.authFactory.Address(),
				Value: 1,
				Memo:  binary.BigEndian.AppendUint64(nil, t.currBalance),
			},
		},
		t.authFactory,
	)
	if err != nil {
		return nil, err
	}
	if tx.MaxFee() > t.currBalance {
		return nil, ErrTxGeneratorFundsExhausted
	}
	t.currBalance -= tx.MaxFee()
	return tx, nil
}

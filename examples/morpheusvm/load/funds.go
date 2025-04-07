// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/fees"
)

const txCheckInterval = 100 * time.Millisecond

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrTxFailed          = errors.New("transaction failed")
)

func FundDistributor(
	ctx context.Context,
	uri string,
	funder chain.AuthFactory,
	numAccounts uint64,
) ([]chain.AuthFactory, error) {
	lcli := vm.NewJSONRPCClient(uri)
	cli := jsonrpc.NewJSONRPCClient(uri)
	indexerCli := indexer.NewClient(uri)

	ruleFactory, err := lcli.GetRuleFactory(ctx)
	if err != nil {
		return nil, err
	}
	balance, err := lcli.Balance(ctx, funder.Address())
	if err != nil {
		return nil, err
	}

	unitPrices, err := cli.UnitPrices(ctx, false)
	if err != nil {
		return nil, err
	}

	// Create arbitrary action to estimate units
	action := []chain.Action{&actions.Transfer{
		To:    funder.Address(),
		Value: 1,
		Memo:  binary.BigEndian.AppendUint64(nil, balance),
	}}
	units, err := chain.EstimateUnits(
		ruleFactory.GetRules(time.Now().UnixMilli()),
		action,
		funder,
	)
	if err != nil {
		return nil, err
	}

	fee, err := fees.MulSum(units, unitPrices)
	if err != nil {
		return nil, err
	}

	amountPerAccount := (balance - (numAccounts * fee)) / numAccounts
	if amountPerAccount == 0 {
		return nil, ErrInsufficientFunds
	}

	// Generate test accounts
	accounts := make([]chain.AuthFactory, numAccounts)
	for i := range numAccounts {
		pk, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		accounts[i] = auth.NewED25519Factory(pk)
	}

	// Send and confirm funds to each account
	for _, account := range accounts {
		action := &actions.Transfer{
			To:    account.Address(),
			Value: amountPerAccount,
			Memo:  binary.BigEndian.AppendUint64(nil, balance),
		}
		tx, err := chain.GenerateTransaction(
			ruleFactory,
			unitPrices,
			[]chain.Action{action},
			funder,
		)
		if err != nil {
			return nil, err
		}
		txID, err := cli.SubmitTx(ctx, tx.Bytes())
		if err != nil {
			return nil, err
		}

		success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, txID)
		if err != nil {
			return nil, err
		}
		if !success {
			return nil, ErrTxFailed
		}
	}
	return accounts, nil
}

func FundConsolidator(
	ctx context.Context,
	uri string,
	accounts []chain.AuthFactory,
	to chain.AuthFactory,
) error {
	lcli := vm.NewJSONRPCClient(uri)
	cli := jsonrpc.NewJSONRPCClient(uri)
	indexerCli := indexer.NewClient(uri)

	ruleFactory, err := lcli.GetRuleFactory(ctx)
	if err != nil {
		return err
	}

	unitPrices, err := cli.UnitPrices(ctx, false)
	if err != nil {
		return err
	}

	action := []chain.Action{&actions.Transfer{
		To:    to.Address(),
		Value: 1,
		Memo:  binary.BigEndian.AppendUint64(nil, 1),
	}}

	units, err := chain.EstimateUnits(
		ruleFactory.GetRules(time.Now().UnixMilli()),
		action,
		to,
	)
	if err != nil {
		return err
	}

	fee, err := fees.MulSum(units, unitPrices)
	if err != nil {
		return err
	}

	balances := make([]uint64, len(accounts))
	for i, account := range accounts {
		balance, err := lcli.Balance(ctx, account.Address())
		if err != nil {
			return err
		}
		balances[i] = balance
	}

	txs := make([]*chain.Transaction, 0)
	for i, balance := range balances {
		if balance < fee {
			continue
		}
		amount := balance - fee
		action := &actions.Transfer{
			To:    to.Address(),
			Value: amount,
			Memo:  binary.BigEndian.AppendUint64(nil, amount),
		}
		tx, err := chain.GenerateTransaction(
			ruleFactory,
			unitPrices,
			[]chain.Action{action},
			accounts[i],
		)
		if err != nil {
			return err
		}
		txs = append(txs, tx)
	}

	for _, tx := range txs {
		txID, err := cli.SubmitTx(ctx, tx.Bytes())
		if err != nil {
			return err
		}

		success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, txID)
		if err != nil {
			return err
		}
		if !success {
			return ErrTxFailed
		}
	}

	return nil
}

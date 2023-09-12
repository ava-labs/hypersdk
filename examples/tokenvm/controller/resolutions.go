// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/challenge"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/orderbook"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	tutils "github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

func (c *Controller) Genesis() *genesis.Genesis {
	return c.genesis
}

func (c *Controller) Logger() logging.Logger {
	return c.inner.Logger()
}

func (c *Controller) Tracer() trace.Tracer {
	return c.inner.Tracer()
}

func (c *Controller) GetTransaction(
	ctx context.Context,
	txID ids.ID,
) (bool, int64, bool, chain.Dimensions, uint64, error) {
	return storage.GetTransaction(ctx, c.metaDB, txID)
}

func (c *Controller) GetAssetFromState(
	ctx context.Context,
	asset ids.ID,
) (bool, []byte, uint8, []byte, uint64, ed25519.PublicKey, bool, error) {
	return storage.GetAssetFromState(ctx, c.inner.ReadState, asset)
}

func (c *Controller) GetBalanceFromState(
	ctx context.Context,
	pk ed25519.PublicKey,
	asset ids.ID,
) (uint64, error) {
	return storage.GetBalanceFromState(ctx, c.inner.ReadState, pk, asset)
}

func (c *Controller) Orders(pair string, limit int) []*orderbook.Order {
	return c.orderBook.Orders(pair, limit)
}

func (c *Controller) GetLoanFromState(
	ctx context.Context,
	asset ids.ID,
	destination ids.ID,
) (uint64, error) {
	return storage.GetLoanFromState(ctx, c.inner.ReadState, asset, destination)
}

func (c *Controller) GetFaucetAddress(_ context.Context) (ed25519.PublicKey, error) {
	if c.config.GetFaucetAmount() == 0 {
		return ed25519.EmptyPublicKey, errors.New("faucet disabled")
	}
	return c.faucetKey.PublicKey(), nil
}

func (c *Controller) GetChallenge(_ context.Context) ([]byte, uint16, error) {
	if c.config.GetFaucetAmount() == 0 {
		return nil, 0, errors.New("faucet disabled")
	}

	c.saltLock.RLock()
	defer c.saltLock.RUnlock()
	return c.salt, c.config.GetFaucetDifficulty(), nil
}

func (c *Controller) sendFunds(ctx context.Context, destination ed25519.PublicKey, amount uint64) (ids.ID, uint64, error) {
	bal, err := c.GetBalanceFromState(ctx, c.faucetKey.PublicKey(), ids.Empty)
	if err != nil {
		return ids.Empty, 0, err
	}

	unitPrices, err := c.inner.UnitPrices(ctx)
	if err != nil {
		return ids.Empty, 0, err
	}

	action := &actions.Transfer{
		To:    destination,
		Asset: ids.Empty,
		Value: amount,
	}
	factory := auth.NewED25519Factory(c.faucetKey)
	now := time.Now().UnixMilli()
	rules := c.inner.Rules(now)
	maxUnits, err := chain.EstimateMaxUnits(rules, action, factory, nil)
	if err != nil {
		return ids.Empty, 0, err
	}
	maxFee, err := chain.MulSum(unitPrices, maxUnits)
	if err != nil {
		return ids.Empty, 0, err
	}
	if bal < maxFee+amount {
		c.snowCtx.Log.Warn("faucet has insufficient funds", zap.String("balance", utils.FormatBalance(bal, consts.Decimals)))
		return ids.Empty, 0, errors.New("insufficient balance")
	}
	base := &chain.Base{
		Timestamp: utils.UnixRMilli(now, rules.GetValidityWindow()),
		ChainID:   rules.ChainID(),
		MaxFee:    maxFee,
	}
	tx := chain.NewTx(base, nil, action)
	signedTx, err := tx.Sign(factory, consts.ActionRegistry, consts.AuthRegistry)
	if err != nil {
		return ids.Empty, 0, err
	}
	errs := c.inner.Submit(ctx, false, []*chain.Transaction{signedTx})
	return signedTx.ID(), maxFee, errs[0]
}

// TODO: increase difficulty if solutions/minute greater than target
func (c *Controller) SolveChallenge(ctx context.Context, solver ed25519.PublicKey, salt []byte, solution []byte) (ids.ID, error) {
	if c.config.GetFaucetAmount() == 0 {
		return ids.Empty, errors.New("faucet disabled")
	}

	// Ensure solution is valid
	c.saltLock.Lock()
	defer c.saltLock.Unlock()
	if !bytes.Equal(c.salt, salt) {
		return ids.Empty, errors.New("salt expired")
	}
	if !challenge.Verify(salt, solution, c.config.GetFaucetDifficulty()) {
		return ids.Empty, errors.New("invalid solution")
	}
	solutionID := utils.ToID(solution)
	if c.solutions.Contains(solutionID) {
		return ids.Empty, errors.New("duplicate solution")
	}

	// Issue transaction
	txID, maxFee, err := c.sendFunds(ctx, solver, c.config.GetFaucetAmount())
	if err != nil {
		return ids.Empty, err
	}
	c.snowCtx.Log.Info("fauceted funds",
		zap.Stringer("txID", txID),
		zap.String("max fee", utils.FormatBalance(maxFee, consts.Decimals)),
		zap.String("destination", tutils.Address(c.faucetKey.PublicKey())),
		zap.String("amount", utils.FormatBalance(c.config.GetFaucetAmount(), consts.Decimals)),
	)
	c.solutions.Add(solutionID)

	// Roll salt if stale
	if c.solutions.Len() < c.config.GetFaucetSolutionsPerSalt() {
		return txID, nil
	}
	c.salt, err = challenge.New()
	if err != nil {
		// Should never happen
		return ids.Empty, err
	}
	c.solutions.Clear()
	return txID, nil
}

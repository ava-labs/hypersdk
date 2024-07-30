// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/lineagevm/genesis"
	"github.com/ava-labs/hypersdk/examples/lineagevm/storage"
	"github.com/ava-labs/hypersdk/fees"
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
) (bool, int64, bool, fees.Dimensions, uint64, error) {
	return storage.GetTransaction(ctx, c.metaDB, txID)
}

func (c *Controller) GetBalanceFromState(
	ctx context.Context,
	acct codec.Address,
) (uint64, error) {
	return storage.GetBalanceFromState(ctx, c.inner.ReadState, acct)
}

func (c *Controller) GetProfessorFromState(
	ctx context.Context,
	professorID codec.Address,
) (success bool, name string, year uint16, university string, students []codec.Address, err error) {
	return storage.GetProfessorFromStateComplex(ctx, c.inner.ReadState, professorID)
}

func (c *Controller) DoesLineageExist(
	ctx context.Context,
	professorName string,
	studentName string,
) (existsPath bool, err error) {
	return storage.DoesLineageExist(ctx, c.inner.ReadState, professorName, studentName)
}

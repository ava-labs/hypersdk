// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

// NewGenesisCommit returns both a block and a view that represents the
// genesis state transition.
func NewGenesisCommit(
	ctx context.Context,
	view merkledb.View,
	genesis Genesis,
	metadataManager MetadataManager,
	balanceHandler BalanceHandler,
	ruleFactory RuleFactory,
	tracer trace.Tracer,
	logger logging.Logger,
) (*ExecutionBlock, merkledb.View, error) {
	ts := tstate.New(0)
	tsv := ts.NewView(state.CompletePermissions, view, 0)
	if err := genesis.InitializeState(ctx, tracer, tsv, balanceHandler); err != nil {
		return nil, nil, fmt.Errorf("failed to initialize genesis state: %w", err)
	}

	// Update chain metadata
	if err := tsv.Insert(ctx, HeightKey(metadataManager.HeightPrefix()), binary.BigEndian.AppendUint64(nil, 0)); err != nil {
		return nil, nil, fmt.Errorf("failed to set genesis height: %w", err)
	}
	if err := tsv.Insert(ctx, TimestampKey(metadataManager.TimestampPrefix()), binary.BigEndian.AppendUint64(nil, 0)); err != nil {
		return nil, nil, fmt.Errorf("failed to set genesis timestamp: %w", err)
	}
	genesisRules := ruleFactory.GetRules(0)
	feeManager := internalfees.NewManager(nil)
	minUnitPrice := genesisRules.GetMinUnitPrice()
	for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
		feeManager.SetUnitPrice(i, minUnitPrice[i])
		logger.Info("set genesis unit price", zap.Int("dimension", int(i)), zap.Uint64("price", feeManager.UnitPrice(i)))
	}
	if err := tsv.Insert(ctx, FeeKey(metadataManager.FeePrefix()), feeManager.Bytes()); err != nil {
		return nil, nil, fmt.Errorf("failed to set genesis fee manager: %w", err)
	}

	// Commit genesis block post-execution state and compute root
	tsv.Commit()
	genesisView, err := view.NewView(ctx, merkledb.ViewChanges{
		MapOps:       ts.ChangedKeys(),
		ConsumeBytes: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to commit genesis initialized state diff: %w", err)
	}

	root, err := genesisView.GetMerkleRoot(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get initialized genesis root: %w", err)
	}

	// We set the genesis block timestamp to be after the ProposerVM fork activation.
	//
	// This prevents an issue (when using millisecond timestamps) during ProposerVM activation
	// where the child timestamp is rounded down to the nearest second (which may be before
	// the timestamp of its parent, which is denoted in milliseconds).
	//
	// Link: https://github.com/ava-labs/avalanchego/blob/0ec52a9c6e5b879e367688db01bb10174d70b212
	// .../vms/proposervm/pre_fork_block.go#L201
	sb, err := NewStatelessBlock(
		ids.Empty,
		time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		0,
		nil,
		root, // StateRoot should include all allocates made when loading the genesis file
		nil,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create genesis stateless block: %w", err)
	}

	return NewExecutionBlock(sb), genesisView, nil
}

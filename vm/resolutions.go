// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/builder"
	"github.com/ava-labs/hypersdk/internal/gossiper"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var (
	_ gossiper.ValidatorSet                         = (*VM)(nil)
	_ validitywindow.ChainIndex[*chain.Transaction] = (*VM)(nil)
)

func (vm *VM) ChainID() ids.ID {
	return vm.snowCtx.ChainID
}

func (vm *VM) NetworkID() uint32 {
	return vm.snowCtx.NetworkID
}

func (vm *VM) SubnetID() ids.ID {
	return vm.snowCtx.SubnetID
}

func (vm *VM) ActionCodec() *codec.TypeParser[chain.Action] {
	return vm.actionCodec
}

func (vm *VM) OutputCodec() *codec.TypeParser[codec.Typed] {
	return vm.outputCodec
}

func (vm *VM) AuthCodec() *codec.TypeParser[chain.Auth] {
	return vm.authCodec
}

func (vm *VM) AuthVerifiers() workers.Workers {
	return vm.authVerifiers
}

func (vm *VM) RuleFactory() chain.RuleFactory {
	return vm.ruleFactory
}

func (vm *VM) Metrics() metrics.MultiGatherer {
	return vm.snowCtx.Metrics
}

func (vm *VM) Tracer() trace.Tracer {
	return vm.tracer
}

func (vm *VM) Logger() logging.Logger {
	return vm.snowCtx.Log
}

func (vm *VM) Rules(t int64) chain.Rules {
	return vm.ruleFactory.GetRules(t)
}

func (vm *VM) GetExecutionBlock(ctx context.Context, blkID ids.ID) (validitywindow.ExecutionBlock[*chain.Transaction], error) {
	_, span := vm.tracer.Start(ctx, "VM.GetExecutionBlock")
	defer span.End()

	blk, err := vm.consensusIndex.GetBlock(ctx, blkID)
	if err != nil {
		return nil, err
	}
	return blk, nil
}

func (vm *VM) LastAcceptedBlock(ctx context.Context) (*chain.StatelessBlock, error) {
	outputBlk, err := vm.consensusIndex.GetLastAccepted(ctx)
	if err != nil {
		return nil, err
	}
	return outputBlk.StatelessBlock, nil
}

func (vm *VM) ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error) {
	return vm.stateDB.GetValues(ctx, keys)
}

func (vm *VM) ImmutableState(ctx context.Context) (state.Immutable, error) {
	return vm.stateDB.NewView(ctx, merkledb.ViewChanges{MapOps: nil, ConsumeBytes: true})
}

func (vm *VM) Mempool() chain.Mempool {
	return vm.mempool
}

func (vm *VM) IsValidator(ctx context.Context, nid ids.NodeID) (bool, error) {
	return vm.proposerMonitor.IsValidator(ctx, nid)
}

func (vm *VM) Proposers(ctx context.Context, diff int, depth int) (set.Set[ids.NodeID], error) {
	return vm.proposerMonitor.Proposers(ctx, diff, depth)
}

func (vm *VM) CurrentValidators(
	ctx context.Context,
) (map[ids.NodeID]*validators.GetValidatorOutput, map[string]struct{}) {
	return vm.proposerMonitor.Validators(ctx)
}

func (vm *VM) NodeID() ids.NodeID {
	return vm.snowCtx.NodeID
}

func (vm *VM) PreferredHeight(ctx context.Context) (uint64, error) {
	blk, err := vm.consensusIndex.GetPreferredBlock(ctx)
	if err != nil {
		return 0, err
	}
	return blk.Hght, nil
}

// Used for integration and load testing
func (vm *VM) Builder() builder.Builder {
	return vm.builder
}

func (vm *VM) Gossiper() gossiper.Gossiper {
	return vm.gossiper
}

func (vm *VM) Genesis() genesis.Genesis {
	return vm.genesis
}

func (vm *VM) BalanceHandler() chain.BalanceHandler {
	return vm.balanceHandler
}

func (vm *VM) MetadataManager() chain.MetadataManager {
	return vm.metadataManager
}

func (vm *VM) SubmitTx(ctx context.Context, tx *chain.Transaction) error {
	errs := vm.Submit(ctx, []*chain.Transaction{tx})
	return errs[0]
}

func (vm *VM) GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	bv, ok := vm.authEngine[authTypeID]
	if !ok {
		return nil, false
	}
	return bv.GetBatchVerifier(cores, count), ok
}

func (vm *VM) UnitPrices(context.Context) (fees.Dimensions, error) {
	v, err := vm.stateDB.Get(chain.FeeKey(vm.MetadataManager().FeePrefix()))
	if err != nil {
		return fees.Dimensions{}, err
	}
	return internalfees.NewManager(v).UnitPrices(), nil
}

func (vm *VM) GetDataDir() string {
	return vm.DataDir
}

func (vm *VM) GetGenesisBytes() []byte {
	return vm.GenesisBytes
}

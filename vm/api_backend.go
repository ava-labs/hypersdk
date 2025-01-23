// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"path/filepath"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	internalfees "github.com/ava-labs/hypersdk/internal/fees"
	hsnow "github.com/ava-labs/hypersdk/snow"
	"github.com/ava-labs/hypersdk/state"
)

const APIDataDir = "api"

var _ api.VM = (*APIBackend)(nil)

type ConsensusAPI interface {
	LastAcceptedBlock(ctx context.Context) (*chain.ExecutedBlock, error)
}

type Submitter interface {
	Submit(
		ctx context.Context,
		txs []*chain.Transaction,
	) (errs []error)
}

type APIBackend struct {
	dataDir    string
	chainInput hsnow.ChainInput
	chainDef   ChainDefinition
	stateDB    merkledb.MerkleDB
	submitter  Submitter
	consensus  ConsensusAPI
}

func NewAPIBackend(
	chainInput hsnow.ChainInput,
	chainDef ChainDefinition,
	stateDB merkledb.MerkleDB,
	submitter Submitter,
	consensus ConsensusAPI,
) *APIBackend {
	return &APIBackend{
		dataDir:    filepath.Join(chainInput.SnowCtx.ChainDataDir, APIDataDir),
		chainInput: chainInput,
		chainDef:   chainDef,
		stateDB:    stateDB,
		submitter:  submitter,
		consensus:  consensus,
	}
}

func (a *APIBackend) GetDataDir() string                           { return a.dataDir }
func (a *APIBackend) GetGenesisBytes() []byte                      { return a.chainDef.GenesisBytes }
func (a *APIBackend) Genesis() genesis.Genesis                     { return a.chainDef.Genesis }
func (a *APIBackend) ChainID() ids.ID                              { return a.chainInput.SnowCtx.ChainID }
func (a *APIBackend) NetworkID() uint32                            { return a.chainInput.SnowCtx.NetworkID }
func (a *APIBackend) SubnetID() ids.ID                             { return a.chainInput.SnowCtx.SubnetID }
func (a *APIBackend) Tracer() trace.Tracer                         { return a.chainInput.Tracer }
func (a *APIBackend) Logger() logging.Logger                       { return a.chainInput.SnowCtx.Log }
func (a *APIBackend) ActionCodec() *codec.TypeParser[chain.Action] { return a.chainDef.actionCodec }
func (a *APIBackend) OutputCodec() *codec.TypeParser[codec.Typed]  { return a.chainDef.outputCodec }
func (a *APIBackend) AuthCodec() *codec.TypeParser[chain.Auth]     { return a.chainDef.authCodec }
func (a *APIBackend) Rules(t int64) chain.Rules                    { return a.chainDef.RuleFactory.GetRules(t) }
func (a *APIBackend) BalanceHandler() chain.BalanceHandler         { return a.chainDef.BalanceHandler }

func (a *APIBackend) ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error) {
	return a.stateDB.GetValues(ctx, keys)
}

func (a *APIBackend) ImmutableState(ctx context.Context) (state.Immutable, error) {
	return a.stateDB.NewView(ctx, merkledb.ViewChanges{MapOps: nil, ConsumeBytes: true})
}

func (a *APIBackend) UnitPrices(context.Context) (fees.Dimensions, error) {
	v, err := a.stateDB.Get(chain.FeeKey(a.chainDef.MetadataManager.FeePrefix()))
	if err != nil {
		return fees.Dimensions{}, err
	}
	return internalfees.NewManager(v).UnitPrices(), nil
}

func (a *APIBackend) Submit(
	ctx context.Context,
	txs []*chain.Transaction,
) (errs []error) {
	return a.submitter.Submit(ctx, txs)
}

func (a *APIBackend) LastAcceptedBlock(ctx context.Context) (*chain.StatelessBlock, error) {
	executedBlock, err := a.consensus.LastAcceptedBlock(ctx)
	if err != nil {
		return nil, err
	}
	return executedBlock.Block, nil
}

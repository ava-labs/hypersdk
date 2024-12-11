// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/enginetest"
	"github.com/ava-labs/avalanchego/snow/snowtest"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

var (
	_ Block                                     = (*TestBlock)(nil)
	_ Chain[*TestBlock, *TestBlock, *TestBlock] = (*TestChain)(nil)
)

var (
	parseInvalidBlockErr    = errors.New("parsed invalid block")
	executedInvalidBlockErr = errors.New("executed invalid block")
)

const blockStringerF = "(ParentID = %s, Timestamp = %d, StateRoot = %d, Height = %d, RandomData = %x, Invalid = %t)"

type TestBlock struct {
	PrntID     ids.ID `serialize:"true" json:"parentID"`
	Tmstmp     int64  `serialize:"true" json:"timestamp"`
	Root       ids.ID `serialize:"true" json:"stateRoot"`
	Hght       uint64 `serialize:"true" json:"height"`
	RandomData []byte `serialize:"true" json:"randomData"`

	// Invalid marks a block that should return an error during execution.
	// This should make it easy to construct a block that should fail execution.
	Invalid bool `serialize:"true" json:"invalid"`
}

func NewTestBlockFromParent(parent *TestBlock, randomData []byte) *TestBlock {
	return &TestBlock{
		PrntID:     parent.ID(),
		Tmstmp:     parent.Timestamp() + 1,
		Hght:       parent.Height() + 1,
		Root:       parent.GetStateRoot(),
		RandomData: randomData,
	}
}

func (t *TestBlock) ID() ids.ID {
	return hashing.ComputeHash256Array(t.Bytes())
}

func (t *TestBlock) Parent() ids.ID {
	return t.PrntID
}

func (t *TestBlock) Timestamp() int64 {
	return t.Tmstmp
}

func (t *TestBlock) Bytes() []byte {
	b, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	return b
}

func (t *TestBlock) GetStateRoot() ids.ID {
	return t.Root
}

func (t *TestBlock) Height() uint64 {
	return t.Hght
}

func (t *TestBlock) String() string {
	return fmt.Sprintf(blockStringerF, t.PrntID, t.Tmstmp, t.Root, t.Hght, t.RandomData, t.Invalid)
}

func NewTestBlockFromBytes(b []byte) (*TestBlock, error) {
	blk := &TestBlock{}
	if err := json.Unmarshal(b, blk); err != nil {
		return nil, err
	}
	return blk, nil
}

type VerifiedBlock struct {
	*TestBlock
	verified bool
}

type AcceptedBlock struct {
	*VerifiedBlock
	accepted bool
}

type TestChain struct {
	t                     *testing.T
	initLastAcceptedBlock *TestBlock
	getRandData           func() []byte

	acceptedBlocks         map[ids.ID]*TestBlock
	acceptedBlocksByHeight map[uint64]*TestBlock
}

func NewTestChain(t *testing.T, getRandData func() []byte, initLastAcceptedBlock *TestBlock) *TestChain {
	return &TestChain{
		t:                      t,
		getRandData:            getRandData,
		initLastAcceptedBlock:  initLastAcceptedBlock,
		acceptedBlocks:         make(map[ids.ID]*TestBlock),
		acceptedBlocksByHeight: make(map[uint64]*TestBlock),
	}
}

func (t *TestChain) Initialize(
	ctx context.Context,
	chainInput ChainInput,
	chainIndex ChainIndex[*TestBlock, *TestBlock, *TestBlock],
	options *Options[*TestBlock, *TestBlock, *TestBlock],
) (*TestBlock, *TestBlock, *TestBlock, error) {
	return t.initLastAcceptedBlock, t.initLastAcceptedBlock, t.initLastAcceptedBlock, nil
}

func (t *TestChain) BuildBlock(ctx context.Context, parent *TestBlock) (*TestBlock, *TestBlock, error) {
	builtBlock := NewTestBlockFromParent(parent, t.getRandData())
	return builtBlock, builtBlock, nil

}

func (t *TestChain) ParseBlock(ctx context.Context, bytes []byte) (*TestBlock, error) {
	return NewTestBlockFromBytes(bytes)
}

func (t *TestChain) Execute(ctx context.Context, parent *TestBlock, block *TestBlock) (*TestBlock, error) {
	if parent.Invalid {
		return nil, fmt.Errorf("%w: %s", executedInvalidBlockErr, block)
	}
	return block, nil
}

func (t *TestChain) AcceptBlock(ctx context.Context, verifiedBlock *TestBlock) (*TestBlock, error) {
	t.acceptedBlocks[verifiedBlock.ID()] = verifiedBlock
	t.acceptedBlocksByHeight[verifiedBlock.Height()] = verifiedBlock
	return verifiedBlock, nil
}

func (t *TestChain) GetBlock(ctx context.Context, blkID ids.ID) ([]byte, error) {
	blk, ok := t.acceptedBlocks[blkID]
	if !ok {
		return nil, fmt.Errorf("block not found")
	}
	return blk.Bytes(), nil
}

func (t *TestChain) GetBlockIDAtHeight(ctx context.Context, blkHeight uint64) (ids.ID, error) {
	blk, ok := t.acceptedBlocksByHeight[blkHeight]
	if !ok {
		return ids.ID{}, fmt.Errorf("block not found")
	}
	return blk.ID(), nil
}

type TestConsensusEngine struct {
	t     *testing.T
	r     *require.Assertions
	chain *TestChain
	vm    *VM[*TestBlock, *TestBlock, *TestBlock]

	lastAccepted *StatefulBlock[*TestBlock, *TestBlock, *TestBlock]
	preferred    *StatefulBlock[*TestBlock, *TestBlock, *TestBlock]
	verified     map[ids.ID]*StatefulBlock[*TestBlock, *TestBlock, *TestBlock]
	children     map[ids.ID]set.Set[ids.ID]
}

func NewTestConsensusEngine(t *testing.T, initLastAcceptedBlock *TestBlock) *TestConsensusEngine {
	r := require.New(t)
	ctx := context.Background()
	rand := rand.New(rand.NewSource(0))
	getRandData := func() []byte {
		data := make([]byte, 32)
		_, err := rand.Read(data)
		if err != nil {
			panic(err)
		}
		return data
	}
	chain := NewTestChain(t, getRandData, initLastAcceptedBlock)
	vm := NewVM(chain)
	toEngine := make(chan common.Message, 1)
	ce := &TestConsensusEngine{
		t:        t,
		r:        r,
		chain:    chain,
		vm:       vm,
		verified: make(map[ids.ID]*StatefulBlock[*TestBlock, *TestBlock, *TestBlock]),
		children: make(map[ids.ID]set.Set[ids.ID]),
	}
	r.NoError(vm.Initialize(ctx, snowtest.Context(t, ids.GenerateTestID()), nil, nil, nil, nil, toEngine, nil, &enginetest.Sender{T: t}))
	ce.lastAccepted = vm.covariantVM.LastAcceptedBlock(ctx)
	ce.preferred = ce.lastAccepted
	return ce
}

// BuildBlock copies the expected behavior of the consensus engine when building a block
// and assumes the VM always builds a correct block.
func (ce *TestConsensusEngine) BuildBlock(ctx context.Context) (*StatefulBlock[*TestBlock, *TestBlock, *TestBlock], bool) {
	preferredID := ce.preferred.ID()
	blk, err := ce.vm.covariantVM.BuildBlock(ctx)

	ce.r.NoError(err)
	ce.r.Equal(preferredID, blk.Parent())
	// Skip if we built a block identical to one we've already verified
	if _, ok := ce.verified[blk.ID()]; ok {
		return blk, false
	}

	ce.verifyValidBlock(ctx, blk)
	ce.r.NoError(blk.Verify(ctx))
	ce.verified[blk.ID()] = blk

	// Note: there is technically a case in the engine where building a block can enable issuance of
	// pending blocks that are missing an ancestor. We ignore this edge case for simplicity here.
	ce.r.NoError(ce.vm.SetPreference(ctx, blk.ID()))
	ce.preferred = blk
	return blk, true
}

func (ce *TestConsensusEngine) verifyValidBlock(ctx context.Context, blk *StatefulBlock[*TestBlock, *TestBlock, *TestBlock]) {
	ce.r.NoError(blk.Verify(ctx))
	ce.verified[blk.ID()] = blk

	children, ok := ce.children[blk.Parent()]
	if !ok {
		children = set.NewSet[ids.ID](1)
		ce.children[blk.Parent()] = children
	}
	children.Add(blk.ID())
}

// getLastAcceptedToBlk returns the chain of blocks in the range (lastAcceptedBlk, blk]
// If lastAcceptedBlk == blk, this returns an empty chain
// Assumes that blk and its ancestors tracing back to lastAcceptedBlk are in verified
func (ce *TestConsensusEngine) getLastAcceptedToBlk(ctx context.Context, blk *StatefulBlock[*TestBlock, *TestBlock, *TestBlock]) []*StatefulBlock[*TestBlock, *TestBlock, *TestBlock] {
	if blk.ID() == ce.lastAccepted.ID() {
		return nil
	}

	chain := make([]*StatefulBlock[*TestBlock, *TestBlock, *TestBlock], 0)
	for {
		// Add the block to the chain and check for the next block
		chain = append(chain, blk)
		if parentBlk, ok := ce.verified[blk.Parent()]; ok {
			blk = parentBlk
			continue
		}

		if blk.Parent() == ce.lastAccepted.ID() {
			break
		}
		ce.r.FailNow("could not find parent tracing to last accepted block")
	}
	slices.Reverse(chain)
	return chain
}

// acceptChain should mimic the accept behavior of acceptPreferredChild in the Snow consensus engine
// Ref. https://github.com/ava-labs/avalanchego/blob/f6a5c1cd9e0fce911fb2367d1e69b8bb9af1fceb/snow/consensus/snowman/topological.go#L578
func (ce *TestConsensusEngine) acceptChain(ctx context.Context, chain []*StatefulBlock[*TestBlock, *TestBlock, *TestBlock]) {
	for _, blk := range chain {
		_, ok := ce.verified[blk.ID()]
		ce.r.True(ok)

		parent := ce.lastAccepted
		ce.r.Equal(parent.ID(), blk.Parent())

		ce.r.NoError(blk.Accept(ctx))
		delete(ce.verified, blk.ID())
		ce.lastAccepted = blk

		children := ce.children[parent.ID()]
		children.Remove(blk.ID())
		delete(ce.children, parent.ID())

		ce.rejectTransitively(ctx, children)
	}
}

func (ce *TestConsensusEngine) rejectTransitively(ctx context.Context, toReject set.Set[ids.ID]) {
	for child := range toReject {
		childBlk, ok := ce.verified[child]
		ce.r.True(ok)
		ce.r.NoError(childBlk.Reject(ctx))
		delete(ce.verified, child)

		ce.rejectTransitively(ctx, ce.children[child])
	}
}

func (ce *TestConsensusEngine) AcceptPreferredChain(ctx context.Context) (*StatefulBlock[*TestBlock, *TestBlock, *TestBlock], bool) {
	preferredChain := ce.getLastAcceptedToBlk(ctx, ce.preferred)

	ce.acceptChain(ctx, preferredChain)
	if len(preferredChain) == 0 {
		return nil, false
	}
	return preferredChain[len(preferredChain)-1], true
}

func (ce *TestConsensusEngine) ParseInvalidBlockBytes(ctx context.Context) {
	_, err := ce.vm.ParseBlock(ctx, utils.RandomBytes(100))
	ce.r.ErrorIs(err, parseInvalidBlockErr)
}

func (ce *TestConsensusEngine) ParseFutureBlock(ctx context.Context) {
	tBlk := &TestBlock{
		PrntID: ids.GenerateTestID(),
		Tmstmp: math.MaxInt64,
		Hght:   math.MaxUint64,
	}
	blk, err := ce.vm.ParseBlock(ctx, tBlk.Bytes())
	ce.r.NoError(err)
	ce.r.Equal(tBlk.ID(), blk.ID())
	ce.r.Equal(tBlk.Parent(), blk.Parent())
	ce.r.Equal(time.UnixMilli(tBlk.Timestamp()), blk.Timestamp())
	ce.r.Equal(tBlk.Height(), blk.Height())
}

func (ce *TestConsensusEngine) ParseAndVerifyNewBlock(ctx context.Context, parent *StatefulBlock[*TestBlock, *TestBlock, *TestBlock]) *StatefulBlock[*TestBlock, *TestBlock, *TestBlock] {
	newBlk := NewTestBlockFromParent(parent.Input, ce.chain.getRandData())
	parsedBlk, err := ce.vm.covariantVM.ParseBlock(ctx, newBlk.Bytes())
	ce.r.NoError(err)
	ce.r.Equal(newBlk.ID(), parsedBlk.ID())
	ce.verifyValidBlock(ctx, parsedBlk)
	return parsedBlk
}

func (ce *TestConsensusEngine) ParseAndVerifyNewRandomBlock(ctx context.Context) *StatefulBlock[*TestBlock, *TestBlock, *TestBlock] {
	blk, ok := ce.selectRandomVerifiedBlock(ctx)
	if !ok {
		blk = ce.lastAccepted
	}

	return ce.ParseAndVerifyNewBlock(ctx, blk)
}

func (ce *TestConsensusEngine) ParseVerifiedBlk(ctx context.Context) (*StatefulBlock[*TestBlock, *TestBlock, *TestBlock], bool) {
	blk, ok := ce.selectRandomVerifiedBlock(ctx)
	if !ok {
		return nil, false
	}

	parsedBlk, err := ce.vm.ParseBlock(ctx, blk.Bytes())
	ce.r.NoError(err)
	ce.r.Equal(blk.ID(), parsedBlk.ID())
	ce.r.Equal(blk, parsedBlk)
	return blk, true
}

func (ce *TestConsensusEngine) ParseAndVerifyInvalidBlock(ctx context.Context) {
	blk, ok := ce.selectRandomVerifiedBlock(ctx)
	if !ok {
		blk = ce.lastAccepted
	}

	newBlk := NewTestBlockFromParent(blk.Input, ce.chain.getRandData())
	newBlk.Invalid = true
	parsedBlk, err := ce.vm.ParseBlock(ctx, newBlk.Bytes())
	ce.r.NoError(err)
	ce.r.Equal(newBlk.ID(), parsedBlk.ID())
	ce.r.ErrorIs(parsedBlk.Verify(ctx), executedInvalidBlockErr)
	_, ok = ce.verified[parsedBlk.ID()]
	ce.r.False(ok)
}

func (ce *TestConsensusEngine) SwapRandomPreference(ctx context.Context) (old *StatefulBlock[*TestBlock, *TestBlock, *TestBlock], new *StatefulBlock[*TestBlock, *TestBlock, *TestBlock], changed bool) {
	selectedBlk, ok := ce.selectRandomVerifiedBlock(ctx)
	if !ok {
		selectedBlk = ce.lastAccepted
	}
	old = ce.preferred
	new = selectedBlk
	changed = ce.preferred.ID() != selectedBlk.ID()
	ce.preferred = selectedBlk
	ce.vm.SetPreference(ctx, selectedBlk.ID())
	return old, new, changed
}

func (ce *TestConsensusEngine) SetPreference(ctx context.Context, blkID ids.ID) (old *StatefulBlock[*TestBlock, *TestBlock, *TestBlock], new *StatefulBlock[*TestBlock, *TestBlock, *TestBlock], changed bool) {
	selectedBlk, ok := ce.verified[blkID]
	if !ok && ce.lastAccepted.ID() == blkID {
		selectedBlk = ce.lastAccepted
		ok = true
	}
	ce.r.True(ok)

	old = ce.preferred
	new = selectedBlk
	changed = ce.preferred.ID() != selectedBlk.ID()
	ce.preferred = selectedBlk
	ce.vm.SetPreference(ctx, selectedBlk.ID())
	return old, new, changed
}

func (ce *TestConsensusEngine) AcceptNonPreferredBlock(ctx context.Context) {
	preferredChain := ce.getLastAcceptedToBlk(ctx, ce.preferred)
	preferredSet := set.NewSet[ids.ID](len(preferredChain))
	for _, blk := range preferredChain {
		preferredSet.Add(blk.ID())
	}

	var selectedBlk *StatefulBlock[*TestBlock, *TestBlock, *TestBlock]
	for _, blk := range ce.verified {
		if !preferredSet.Contains(blk.ID()) {
			selectedBlk = blk
			break
		}
	}
	if selectedBlk == nil {
		ce.t.Log("no non-preferred block to accept")
	}

	nonPreferredChain := ce.getLastAcceptedToBlk(ctx, selectedBlk)
	ce.acceptChain(ctx, nonPreferredChain)
}

func (ce *TestConsensusEngine) GetVerifiedBlock(ctx context.Context) (*StatefulBlock[*TestBlock, *TestBlock, *TestBlock], bool) {
	selectedBlk, ok := ce.selectRandomVerifiedBlock(ctx)
	if !ok {
		return nil, false
	}

	blk, err := ce.vm.GetBlock(ctx, selectedBlk.ID())
	ce.r.NoError(err)
	ce.r.Equal(blk, selectedBlk)
	return selectedBlk, true
}

func (ce *TestConsensusEngine) GetLastAcceptedBlock(ctx context.Context) {
	blk, err := ce.vm.GetBlock(ctx, ce.lastAccepted.ID())
	ce.r.NoError(err)
	ce.r.Equal(blk.ID(), ce.lastAccepted.ID())
	ce.r.Equal(blk, ce.lastAccepted)
}

// TODO: get accepted historical block

func (ce *TestConsensusEngine) selectRandomVerifiedBlock(ctx context.Context) (*StatefulBlock[*TestBlock, *TestBlock, *TestBlock], bool) {
	for _, blk := range ce.verified {
		return blk, true
	}
	return nil, false
}

func TestBuildAndAcceptBlock(t *testing.T) {
	ctx := context.Background()

	ce := NewTestConsensusEngine(t, &TestBlock{})

	blk1, ok := ce.BuildBlock(ctx)
	ce.r.True(ok)
	ce.r.Equal(uint64(1), blk1.Height())
	ce.r.Equal(uint64(1), ce.preferred.Height())

	acceptedTip, ok := ce.AcceptPreferredChain(ctx)
	ce.r.True(ok)
	ce.r.Equal(acceptedTip.ID(), blk1.ID())

	ce.r.Len(ce.verified, 0)
}

func TestBuildAndAcceptChain(t *testing.T) {
	ctx := context.Background()

	ce := NewTestConsensusEngine(t, &TestBlock{})

	blk1, ok := ce.BuildBlock(ctx)
	ce.r.True(ok)
	ce.r.Equal(uint64(1), blk1.Height())
	blk2, ok := ce.BuildBlock(ctx)
	ce.r.True(ok)
	ce.r.Equal(uint64(2), blk2.Height())
	ce.r.Equal(blk2.ID(), ce.preferred.ID())
	acceptedTip, ok := ce.AcceptPreferredChain(ctx)
	ce.r.True(ok)
	ce.r.Equal(acceptedTip.ID(), blk2.ID())

	ce.r.Len(ce.verified, 0)
}

func TestParseAndAcceptBlock(t *testing.T) {
	ctx := context.Background()

	ce := NewTestConsensusEngine(t, &TestBlock{})

	blk1 := ce.ParseAndVerifyNewRandomBlock(ctx)
	ce.r.Equal(uint64(1), blk1.Height())

	old, new, changed := ce.SetPreference(ctx, blk1.ID())
	ce.r.True(changed)
	ce.r.Equal(ce.lastAccepted, old)
	ce.r.Equal(blk1, new)

	acceptedTip, ok := ce.AcceptPreferredChain(ctx)
	ce.r.True(ok)
	ce.r.Equal(acceptedTip.ID(), blk1.ID())

	ce.r.Len(ce.verified, 0)
}

func TestParseAndAcceptChain(t *testing.T) {
	ctx := context.Background()

	ce := NewTestConsensusEngine(t, &TestBlock{})

	blk0 := ce.lastAccepted
	blk1 := ce.ParseAndVerifyNewRandomBlock(ctx)
	ce.r.Equal(uint64(1), blk1.Height())

	old, new, changed := ce.SetPreference(ctx, blk1.ID())
	ce.r.True(changed)
	ce.r.Equal(blk0, old)
	ce.r.Equal(blk1, new)
	ce.r.Equal(uint64(1), ce.preferred.Height())

	blk2 := ce.ParseAndVerifyNewRandomBlock(ctx)
	ce.r.Equal(uint64(2), blk2.Height())

	old, new, changed = ce.SetPreference(ctx, blk2.ID())
	ce.r.True(changed)
	ce.r.Equal(blk1, old)
	ce.r.Equal(blk2, new)

	acceptedTip, ok := ce.AcceptPreferredChain(ctx)
	ce.r.True(ok)
	ce.r.Equal(acceptedTip.ID(), blk2.ID())

	ce.r.Len(ce.verified, 0)
}

func TestBuild_ParseAndGet_Accept(t *testing.T) {
	ctx := context.Background()

	ce := NewTestConsensusEngine(t, &TestBlock{})

	blk1, ok := ce.BuildBlock(ctx)
	ce.r.True(ok)

	parsedBlk, ok := ce.ParseVerifiedBlk(ctx)
	ce.r.True(ok)
	ce.r.Equal(blk1.ID(), parsedBlk.ID())

	gotBlk, ok := ce.GetVerifiedBlock(ctx)
	ce.r.True(ok)
	ce.r.Equal(blk1.ID(), gotBlk.ID())

	acceptedTip, ok := ce.AcceptPreferredChain(ctx)
	ce.r.True(ok)
	ce.r.Equal(acceptedTip.ID(), blk1.ID())

	ce.r.Len(ce.verified, 0)
}

func TestBuild_ParseAndExtendPreferredChain_Accept(t *testing.T) {
	ctx := context.Background()

	ce := NewTestConsensusEngine(t, &TestBlock{})

	blk1, ok := ce.BuildBlock(ctx)
	ce.r.True(ok)

	blk2 := ce.ParseAndVerifyNewBlock(ctx, blk1)
	ce.r.Equal(uint64(2), blk2.Height())
	ce.SetPreference(ctx, blk2.ID())

	acceptedTip, ok := ce.AcceptPreferredChain(ctx)
	ce.r.True(ok)
	ce.r.Equal(acceptedTip.ID(), blk2.ID())

	ce.r.Len(ce.verified, 0)
}

func TestConflictingChains(t *testing.T) {
	ctx := context.Background()

	ce := NewTestConsensusEngine(t, &TestBlock{})
	genesis := ce.lastAccepted

	builtBlk1, ok := ce.BuildBlock(ctx)
	ce.r.True(ok)
	ce.r.Equal(uint64(1), builtBlk1.Height())

	builtBlk2, ok := ce.BuildBlock(ctx)
	ce.r.True(ok)
	ce.r.Equal(uint64(2), builtBlk2.Height())

	parsedBlk1 := ce.ParseAndVerifyNewBlock(ctx, genesis)
	ce.r.Equal(uint64(1), parsedBlk1.Height())
	parsedBlk2 := ce.ParseAndVerifyNewBlock(ctx, parsedBlk1)
	ce.r.Equal(uint64(2), parsedBlk2.Height())
	ce.SetPreference(ctx, parsedBlk2.ID())

	acceptedTip, ok := ce.AcceptPreferredChain(ctx)
	ce.r.True(ok)
	ce.r.Equal(acceptedTip.ID(), parsedBlk2.ID())

	ce.r.Len(ce.verified, 0)
}

func TestDynamicStateSyncTransition_NoPendingBlocks(t *testing.T) {
	t.Skip()
}

func TestDynamicStateSyncTransition_PendingTree(t *testing.T) {
	t.Skip()
}

func FuzzSnowVM(f *testing.F) {
	f.Skip()
}

func FuzzSnowVMDynamicStateSync(f *testing.F) {
	f.Skip()
}

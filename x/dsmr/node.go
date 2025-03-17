// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
)

const (
	FutureBound = time.Second
)

var (
	ErrNonIncreasingExpiry          = errors.New("expiry must be monotonically increasing")
	ErrInvalidBlockHeight           = errors.New("invalid block height")
	ErrInvalidTimestampNonMonotonic = errors.New("invalid timestamp non-monotonic")
	ErrFutureBlock                  = errors.New("block timestamp too far in the future")
	ErrFutureChunkCert              = errors.New("chunk expiry too far in the future")
	ErrCommitNonValidatorChunk      = errors.New("non-validator attempted to commit chunk")
	ErrEarlyBlock                   = errors.New("block too close to parent timestamp")
	ErrEarlyEmptyBlock              = errors.New("empty block too close to parent timestamp")
	ErrNilBlockContext              = errors.New("nil block context")

	_ HandlerBackend = (*Node)(nil)
)

type Rules struct {
	MinBlockGap                     int64
	MinEmptyBlockGap                int64
	ValidityWindow                  int64
	MaxPendingBandwidthPerValidator uint64
	NetworkID                       uint32
	SubnetID                        ids.ID
	ChainID                         ids.ID
	QuorumNum                       uint64
	QuorumDen                       uint64
}

func NewDefaultRules(networkID uint32, subnetID ids.ID, chainID ids.ID) Rules {
	return Rules{
		MinBlockGap:                     500,        // 500 ms
		MinEmptyBlockGap:                2_000,      // 2s
		ValidityWindow:                  60_000,     // 60x
		MaxPendingBandwidthPerValidator: 50_000_000, // 50 MB
		NetworkID:                       networkID,
		SubnetID:                        subnetID,
		ChainID:                         chainID,
		QuorumNum:                       33, // f + 1
		QuorumDen:                       100,
	}
}

type RuleFactory interface {
	GetRules(timestamp int64) Rules
}

type DefaultRuleFactory struct {
	rules Rules
}

func NewDefaultRuleFactory(networkID uint32, subnetID ids.ID, chainID ids.ID) DefaultRuleFactory {
	return DefaultRuleFactory{
		rules: NewDefaultRules(networkID, subnetID, chainID),
	}
}

func (d DefaultRuleFactory) GetRules(int64) Rules {
	return d.rules
}

type Node struct {
	nodeID ids.NodeID

	clock           mockable.Clock
	chainState      ChainState
	lastChunkExpiry int64
	ruleFactory     RuleFactory

	chunkValidityWindow *validitywindow.TimeValidityWindow[EChunk]

	pendingChunks *pendingChunkStore
	chunkPool     *chunkPool
	networkClient Client
}

func NewNode(
	nodeID ids.NodeID,
	chainState ChainState,
	ruleFactory RuleFactory,
	chunkValidityWindow *validitywindow.TimeValidityWindow[EChunk],
	db database.Database,
	networkClient Client,
) (*Node, error) {
	pendingChunks, err := newPendingChunkStore(db, ruleFactory)
	if err != nil {
		return nil, err
	}

	return &Node{
		nodeID:              nodeID,
		chainState:          chainState,
		ruleFactory:         ruleFactory,
		chunkValidityWindow: chunkValidityWindow,
		pendingChunks:       pendingChunks,
		chunkPool:           newChunkPool(),
		networkClient:       networkClient,
	}, nil
}

func (n *Node) BuildChunk(
	ctx context.Context,
	expiry int64,
	data []byte,
) (*Chunk, *ChunkCertificate, error) {
	// Guarantee that we never produce duplicate chunks by refusing to produce a chunk
	// unless it has a strictly monotonically increasing expiry.
	// This is a minor optimization over using the validity window to check for duplicates.
	// Note: utilizing non-conflicting data is left to the caller.
	if expiry <= n.lastChunkExpiry {
		return nil, nil, ErrNonIncreasingExpiry
	}
	n.lastChunkExpiry = expiry
	unsignedChunk := NewChunk(n.nodeID, expiry, data)

	chunkCert, err := n.networkClient.CollectChunkSignature(ctx, unsignedChunk)
	if err != nil {
		return nil, nil, err
	}
	if err := n.pendingChunks.putPendingChunk(unsignedChunk); err != nil {
		return nil, nil, err
	}
	n.chunkPool.add(chunkCert)
	if err := n.networkClient.BroadcastChunkCertificate(ctx, chunkCert); err != nil {
		return nil, nil, err
	}
	return unsignedChunk, chunkCert, nil
}

func (n *Node) BuildBlock(
	ctx context.Context,
	pChainContext *block.Context,
	parentBlock *Block,
) (*Block, error) {
	chunkCerts := n.chunkPool.gatherChunkCerts()

	eChunkCerts := make([]EChunk, len(chunkCerts))
	for i, cert := range chunkCerts {
		eChunkCerts[i] = EChunk{
			chunkID: cert.Reference.ChunkID,
			expiry:  cert.Reference.Expiry,
		}
	}
	timestamp := n.clock.Time().UnixMilli()

	// TODO: effectively cache container set
	repeatIndices, err := n.chunkValidityWindow.IsRepeat(ctx, newEChunkBlock(parentBlock), timestamp, eChunkCerts)
	if err != nil {
		return nil, err
	}
	selectedChunks := make([]*ChunkCertificate, 0, len(chunkCerts)-repeatIndices.BitLen())
	for i, cert := range chunkCerts {
		if repeatIndices.Contains(i) || cert.Reference.Expiry < timestamp {
			continue
		}

		selectedChunks = append(selectedChunks, cert)
	}

	return NewBlock(
		parentBlock.id,
		parentBlock.Height+1,
		timestamp,
		selectedChunks,
		pChainContext,
	), nil
}

func (n *Node) VerifyBlock(
	ctx context.Context,
	parent *Block,
	block *Block,
) error {
	if block.Height != parent.Height+1 {
		return fmt.Errorf(
			"%w %d: expected %d",
			ErrInvalidBlockHeight,
			block.Height,
			parent.Height+1,
		)
	}

	if block.Timestamp < parent.Timestamp {
		return fmt.Errorf(
			"%w: %d < %d",
			ErrInvalidTimestampNonMonotonic,
			block.Timestamp,
			parent.Timestamp,
		)
	}
	if currentTime := n.clock.Time(); block.Timestamp > currentTime.Add(FutureBound).UnixMilli() {
		return fmt.Errorf(
			"%w: blockTimestamp=%d > currentTime=%d + futureBound=%d",
			ErrFutureBlock,
			block.Timestamp,
			currentTime.UnixMilli(),
			FutureBound.Milliseconds(),
		)
	}

	rules := n.ruleFactory.GetRules(block.Timestamp)
	timestampDiff := block.Timestamp - parent.Timestamp
	if minBlockGap := rules.MinBlockGap; timestampDiff < minBlockGap {
		return fmt.Errorf(
			"%w: %d < %d",
			ErrEarlyBlock,
			timestampDiff,
			rules.MinBlockGap,
		)
	}
	if minEmptyBlockGap := rules.MinEmptyBlockGap; len(block.Chunks) == 0 && timestampDiff < minEmptyBlockGap {
		return fmt.Errorf(
			"%w: %d < %d",
			ErrEarlyEmptyBlock,
			timestampDiff,
			minEmptyBlockGap,
		)
	}

	if err := n.chunkValidityWindow.VerifyExpiryReplayProtection(ctx, newEChunkBlock(block)); err != nil {
		return err
	}

	if block.BlockContext == nil {
		return ErrNilBlockContext
	}
	canonicalVdrSet, err := n.chainState.GetCanonicalValidatorSet(ctx, block.BlockContext.PChainHeight)
	if err != nil {
		return err
	}

	for _, chunkCert := range block.Chunks {
		if err := n.verifyChunkCert(rules, canonicalVdrSet, chunkCert); err != nil {
			return err
		}
	}

	return nil
}

func (n *Node) verifyChunkCert(rules Rules, canonicalVdrSet warp.CanonicalValidatorSet, chunkCert *ChunkCertificate) error {
	var (
		networkID = rules.NetworkID
		chainID   = rules.ChainID
		quorumNum = rules.QuorumNum
		quorumDen = rules.QuorumDen
	)
	unsignedWarpMsg, err := warp.NewUnsignedMessage(networkID, chainID, chunkCert.Reference.bytes)
	if err != nil {
		return err
	}
	return chunkCert.Signature.Verify(
		unsignedWarpMsg,
		networkID,
		canonicalVdrSet,
		quorumNum,
		quorumDen,
	)
}

func (n *Node) AcceptBlock(
	ctx context.Context,
	block *Block,
) error {
	acceptedChunks, err := n.gatherAcceptedChunks(ctx, block.Chunks)
	if err != nil {
		return err
	}

	n.chunkPool.updateHead(block)
	if err := n.pendingChunks.setMin(block.Timestamp, acceptedChunks); err != nil {
		return err
	}
	n.chunkValidityWindow.Accept(newEChunkBlock(block))

	_ = acceptedChunks // TODO: assemble + execute block
	return nil
}

func (n *Node) gatherAcceptedChunks(
	ctx context.Context,
	chunkCerts []*ChunkCertificate,
) ([]*Chunk, error) {
	chunks := make([]*Chunk, len(chunkCerts))
	missingChunkIndices := make([]int, 0, len(chunkCerts)/2)
	missingChunkRefs := make([]*ChunkReference, 0, len(chunkCerts)/2)
	for i, chunkCert := range chunkCerts {
		localChunk, ok := n.pendingChunks.getPendingChunk(chunkCert.Reference.ChunkID)
		if !ok {
			missingChunkIndices = append(missingChunkIndices, i)
			missingChunkRefs = append(missingChunkRefs, chunkCert.Reference)
			continue
		}
		chunks[i] = localChunk
	}

	fetchedChunks, err := n.networkClient.GatherChunks(ctx, missingChunkRefs)
	if err != nil {
		return nil, err
	}
	for i, fetchedChunk := range fetchedChunks {
		chunks[missingChunkIndices[i]] = fetchedChunk
	}

	return chunks, nil
}

func (n *Node) CommitChunk(ctx context.Context, chunk *Chunk) error {
	// Only commit chunks if our current estimate of the next P-Chain height / epoch indicates
	// that node should be allowed to create the chunk.
	// XXX: if the chain pauses, what's the worst that can happen here?
	estimatedPChainHeight, err := n.chainState.EstimatePChainHeight(ctx)
	if err != nil {
		return err
	}
	ok, err := n.chainState.IsNodeValidator(ctx, chunk.Builder, estimatedPChainHeight)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCommitNonValidatorChunk
	}
	timestamp := n.clock.Time().UnixMilli()
	rules := n.ruleFactory.GetRules(timestamp)
	if err := validitywindow.VerifyTimestamp(chunk.Expiry, timestamp, validityWindowTimestampDivisor, rules.ValidityWindow); err != nil {
		return err
	}

	return n.pendingChunks.putPendingChunk(chunk)
}

func (n *Node) AddChunkCert(ctx context.Context, chunkCert *ChunkCertificate) error {
	if currentTime := n.clock.Time(); chunkCert.Reference.Expiry > currentTime.Add(FutureBound).UnixMilli() {
		return fmt.Errorf(
			"%w: chunkCertExpiry=%d > currentTime=%d + futureBound=%d",
			ErrFutureChunkCert,
			chunkCert.Reference.Expiry,
			currentTime.UnixMilli(),
			FutureBound.Milliseconds(),
		)
	}
	rules := n.ruleFactory.GetRules(chunkCert.Reference.Expiry)
	// XXX: should we use estimated P-Chain height, preferred, or last accepted?
	estimatedPChainHeight, err := n.chainState.EstimatePChainHeight(ctx)
	if err != nil {
		return err
	}
	canonicalVdrSet, err := n.chainState.GetCanonicalValidatorSet(ctx, estimatedPChainHeight)
	if err != nil {
		return err
	}
	if err := n.verifyChunkCert(rules, canonicalVdrSet, chunkCert); err != nil {
		return err
	}
	n.chunkPool.add(chunkCert)
	return nil
}

func (n *Node) GetChunkBytes(_ context.Context, chunkRef *ChunkReference) ([]byte, error) {
	return n.pendingChunks.getChunkBytes(chunkRef)
}

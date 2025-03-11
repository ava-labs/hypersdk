// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/crypto/bls"
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
	ErrEmptyBlock                   = errors.New("empty block")
)

type Node struct {
	nodeID    ids.NodeID
	publicKey *bls.PublicKey

	clock           mockable.Clock
	chainState      ChainState
	lastAccepted    *Block // Is this needed?
	lastChunkExpiry int64
	ruleFactory     RuleFactory

	chunkValidityWindow validitywindow.TimeValidityWindow[eChunk]

	pendingChunks *pendingChunkStore
	chunkPool     *chunkPool
	network       P2P
}

func (n *Node) BuildChunk(
	ctx context.Context,
	expiry int64,
	data []byte,
) (*UnsignedChunk, *ChunkCertificate, error) {
	// Guarantee that we never produce duplicate chunks by refusing to produce a chunk
	// unless it has a strictly monotonically increasing expiry.
	// Note: utilizing non-conflicting data is left to the caller.
	if expiry <= n.lastChunkExpiry {
		return nil, nil, ErrNonIncreasingExpiry
	}
	n.lastChunkExpiry = expiry
	unsignedChunk := NewUnsignedChunk(n.nodeID, expiry, data)

	chunkCert, err := n.network.CollectChunkSignature(ctx, unsignedChunk)
	if err != nil {
		return nil, nil, err
	}
	if err := n.pendingChunks.putPendingChunk(unsignedChunk); err != nil {
		return nil, nil, err
	}
	n.chunkPool.add(chunkCert)
	if err := n.network.BroadcastChunkCertificate(ctx, chunkCert); err != nil {
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

	eChunkCerts := make([]eChunk, len(chunkCerts))
	for i, cert := range chunkCerts {
		eChunkCerts[i] = eChunk{
			chunkID: cert.Reference.ChunkID,
			expiry:  cert.Reference.Expiry,
		}
	}
	timestamp := n.clock.Time().UnixMilli()
	repeatIndices, err := n.chunkValidityWindow.IsRepeat(ctx, nil /* TODO */, timestamp, eChunkCerts)
	if err != nil {
		return nil, err
	}
	selectedChunks := make([]ChunkCertificate, 0, len(chunkCerts)-repeatIndices.BitLen())
	for i, cert := range chunkCerts {
		if repeatIndices.Contains(i) || cert.Reference.Expiry < timestamp {
			continue
		}

		selectedChunks = append(selectedChunks, *cert)
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
			"%w: %d > currentTime=%d + futureBound=%d",
			ErrFutureBlock,
			block.Timestamp,
			currentTime.UnixMilli(),
			FutureBound.Milliseconds(),
		)
	}

	// TODO: switch to checking for min time diff for empty block + non-empty block via rules
	if len(block.Chunks) == 0 {
		return fmt.Errorf("%w: %s", ErrEmptyBlock, block.id)
	}

	if err := n.chunkValidityWindow.VerifyExpiryReplayProtection(ctx, newEChunkBlock(block)); err != nil {
		return err
	}

	// TODO: fetch validator set based on height from block context
	canonicalVdrSet, err := n.chainState.GetCanonicalValidatorSet(ctx)
	if err != nil {
		return err
	}

	// make sure all properties are fetched via rules
	for _, chunkCert := range block.Chunks {
		unsignedWarpMsg, err := warp.NewUnsignedMessage(n.chainState.GetNetworkID(), n.chainState.GetChainID(), chunkCert.Reference.bytes)
		if err != nil {
			return err
		}
		if err := chunkCert.Signature.Verify(
			unsignedWarpMsg,
			n.chainState.GetNetworkID(),
			canonicalVdrSet,
			n.chainState.GetQuorumNum(),
			n.chainState.GetQuorumDen(),
		); err != nil {
			return err
		}
	}

	return nil
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
	if err := n.pendingChunks.setMin(block.Timestamp); err != nil {
		return err
	}
	n.lastAccepted = block
	n.chunkValidityWindow.Accept(newEChunkBlock(block))

	_ = acceptedChunks // TODO: assemble + execute block
	return nil
}

func (n *Node) gatherAcceptedChunks(
	ctx context.Context,
	chunkCerts []ChunkCertificate,
) ([]*UnsignedChunk, error) {
	chunks := make([]*UnsignedChunk, len(chunkCerts))
	missingChunkIndices := make([]int, 0, len(chunkCerts)/2)
	missingChunkRefs := make([]ChunkReference, 0, len(chunkCerts)/2)
	for i, chunkCert := range chunkCerts {
		localChunk, ok := n.pendingChunks.getPendingChunk(chunkCert.Reference.ChunkID)
		if !ok {
			missingChunkIndices = append(missingChunkIndices, i)
			missingChunkRefs = append(missingChunkRefs, chunkCert.Reference)
			continue
		}
		chunks[i] = localChunk
	}

	fetchedChunks, err := n.network.GatherChunks(ctx, missingChunkRefs)
	if err != nil {
		return nil, err
	}
	for i, fetchedChunk := range fetchedChunks {
		chunks[missingChunkIndices[i]] = fetchedChunk
	}


	return chunks, nil
}

func (n *Node) CommitChunk(chunk *UnsignedChunk) error {
	return n.pendingChunks.putPendingChunk(chunk)
}

func (n *Node) AddChunkCert(chunkCert *ChunkCertificate) {
	n.chunkPool.add(chunkCert)
}

// Implement server handlers + p2p unit tests
// pass over types and serialization
// update interfaces RuleFactory, ChainState as needed
// persist accepted chunks in accept and implement handler backend for fetching chunks
// integrate into VM
// write package level tests
// update README
// create issue to implement p2p network + main and test outside of VM context

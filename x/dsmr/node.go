// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/acp118"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/emap"
	"github.com/ava-labs/hypersdk/internal/typedclient"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	maxTimeSkew = 30 * time.Second
)

var (
	_ TimeValidityWindow[*emapChunkCertificate] = (validitywindow.Interface[*emapChunkCertificate])(nil)

	ErrEmptyChunk                          = errors.New("empty chunk")
	ErrNoAvailableChunkCerts               = errors.New("no available chunk certs")
	ErrTimestampNotMonotonicallyIncreasing = errors.New("block timestamp must be greater than parent timestamp")
	ErrEmptyBlock                          = errors.New("block must reference chunks")
	ErrInvalidBlockParent                  = errors.New("invalid referenced block parent")
	ErrInvalidBlockHeight                  = errors.New("invalid block height")
	ErrInvalidBlockTimestamp               = errors.New("invalid block timestamp")
	ErrInvalidWarpSignature                = errors.New("invalid warp signature")
	ErrInvalidSignatureType                = errors.New("invalid signature type")
	ErrFailedToReplicate                   = errors.New("failed to replicate to sufficient stake")
)

type ChainState interface {
	GetNetworkID() uint32
	GetSubnetID() ids.ID
	GetChainID() ids.ID
	GetCanonicalValidatorSet(ctx context.Context) (validators warp.CanonicalValidatorSet, err error)
	IsNodeValidator(ctx context.Context, nodeID ids.NodeID, pChainHeight uint64) (bool, error)
	GetQuorumNum() uint64
	GetQuorumDen() uint64
}

type Validator struct {
	NodeID    ids.NodeID
	Weight    uint64
	PublicKey *bls.PublicKey
}

type Rules interface {
	GetValidityWindow() int64
}

type RuleFactory interface {
	GetRules(t int64) Rules
}

func New[T Tx](
	log logging.Logger,
	nodeID ids.NodeID,
	chainState ChainState,
	pk *bls.PublicKey,
	signer warp.Signer,
	chunkStorage *ChunkStorage[T],
	getChunkHandler p2p.Handler,
	getChunkSignatureHandler p2p.Handler,
	chunkCertificateGossipHandler p2p.Handler,
	getChunkClient *p2p.Client,
	getChunkSignatureClient *p2p.Client,
	chunkCertificateGossipClient *p2p.Client,
	lastAccepted Block,
	timeValidityWindow TimeValidityWindow[*emapChunkCertificate],
	ruleFactory RuleFactory,
) (*Node[T], error) {
	return &Node[T]{
		ID:                            nodeID,
		chainState:                    chainState,
		LastAccepted:                  lastAccepted,
		PublicKey:                     pk,
		Signer:                        signer,
		getChunkClient:                NewGetChunkClient[T](getChunkClient),
		chunkCertificateGossipClient:  NewChunkCertificateGossipClient(chunkCertificateGossipClient),
		chunkSignatureAggregator:      acp118.NewSignatureAggregator(log, getChunkSignatureClient),
		GetChunkHandler:               getChunkHandler,
		GetChunkSignatureHandler:      getChunkSignatureHandler,
		ChunkCertificateGossipHandler: chunkCertificateGossipHandler,
		storage:                       chunkStorage,
		log:                           log,
		validityWindow:                timeValidityWindow,
		ruleFactory:                   ruleFactory,
	}, nil
}

type TimeValidityWindow[T emap.Item] interface {
	Accept(blk validitywindow.ExecutionBlock[T])
	VerifyExpiryReplayProtection(
		ctx context.Context,
		blk validitywindow.ExecutionBlock[T],
	) error
	IsRepeat(
		ctx context.Context,
		parentBlk validitywindow.ExecutionBlock[T],
		currentTimestamp int64,
		txs []T,
	) (set.Bits, error)
}

type Node[T Tx] struct {
	ID                           ids.NodeID
	PublicKey                    *bls.PublicKey
	Signer                       warp.Signer
	LastAccepted                 Block
	ruleFactory                  RuleFactory
	getChunkClient               *typedclient.TypedClient[*dsmr.GetChunkRequest, Chunk[T], []byte]
	chunkCertificateGossipClient *typedclient.TypedClient[[]byte, []byte, *dsmr.ChunkCertificateGossip]
	chainState                   ChainState
	chunkSignatureAggregator     *acp118.SignatureAggregator

	GetChunkHandler               p2p.Handler
	GetChunkSignatureHandler      p2p.Handler
	ChunkCertificateGossipHandler p2p.Handler
	storage                       *ChunkStorage[T]
	log                           logging.Logger
	validityWindow                TimeValidityWindow[*emapChunkCertificate]
}

// BuildChunk builds transactions into a Chunk
// TODO handle frozen sponsor + validator assignments
func (n *Node[T]) BuildChunk(
	ctx context.Context,
	txs []T,
	expiry int64,
	beneficiary codec.Address,
) error {
	if len(txs) == 0 {
		return ErrEmptyChunk
	}

	networkID := n.chainState.GetNetworkID()
	chainID := n.chainState.GetChainID()
	chunk, err := signChunk[T](
		UnsignedChunk[T]{
			Producer:    n.ID,
			Beneficiary: beneficiary,
			Expiry:      expiry,
			Txs:         txs,
		},
		networkID,
		chainID,
		n.PublicKey,
		n.Signer,
	)
	if err != nil {
		return fmt.Errorf("failed to sign chunk: %w", err)
	}

	chunkRef := ChunkReference{
		ChunkID:  chunk.id,
		Producer: chunk.Producer,
		Expiry:   chunk.Expiry,
	}
	duplicates, err := n.validityWindow.IsRepeat(ctx, NewValidityWindowBlock(n.LastAccepted), n.LastAccepted.Timestamp, []*emapChunkCertificate{{ChunkCertificate{ChunkReference: chunkRef}}})
	if err != nil {
		return fmt.Errorf("failed to varify repeated chunk certificates : %w", err)
	}
	if duplicates.Len() > 0 {
		// we have duplicates
		return ErrDuplicateChunk
	}

	packer := wrappers.Packer{MaxSize: MaxMessageSize}
	if err := codec.LinearCodec.MarshalInto(chunkRef, &packer); err != nil {
		return fmt.Errorf("failed to marshal chunk reference: %w", err)
	}

	unsignedMsg, err := warp.NewUnsignedMessage(networkID, chainID, packer.Bytes)
	if err != nil {
		return fmt.Errorf("failed to initialize unsigned warp message: %w", err)
	}
	msg, err := warp.NewMessage(
		unsignedMsg,
		&warp.BitSetSignature{
			Signature: [bls.SignatureLen]byte{},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to initialize warp message: %w", err)
	}

	canonicalValidators, err := n.chainState.GetCanonicalValidatorSet(ctx)
	if err != nil {
		return fmt.Errorf("failed to get canonical validator set: %w", err)
	}

	aggregatedMsg, _, _, ok, err := n.chunkSignatureAggregator.AggregateSignatures(
		ctx,
		msg,
		chunk.bytes,
		canonicalValidators.Validators,
		n.chainState.GetQuorumNum(),
		n.chainState.GetQuorumDen(),
	)
	if err != nil {
		return fmt.Errorf("failed to aggregate signatures: %w", err)
	}

	if !ok {
		return ErrFailedToReplicate
	}

	bitSetSignature, ok := aggregatedMsg.Signature.(*warp.BitSetSignature)
	if !ok {
		return ErrInvalidSignatureType
	}

	chunkCert := ChunkCertificate{
		ChunkReference: chunkRef,
		Signature:      bitSetSignature,
	}

	packer = wrappers.Packer{MaxSize: MaxMessageSize}
	if err := codec.LinearCodec.MarshalInto(&chunkCert, &packer); err != nil {
		return err
	}

	if err := n.chunkCertificateGossipClient.AppGossip(
		ctx,
		&dsmr.ChunkCertificateGossip{ChunkCertificate: packer.Bytes},
	); err != nil {
		return err
	}

	return n.storage.AddLocalChunkWithCert(chunk, &chunkCert)
}

func (n *Node[T]) BuildBlock(ctx context.Context, parent Block, timestamp int64) (Block, error) {
	if timestamp <= parent.Timestamp {
		return Block{}, ErrTimestampNotMonotonicallyIncreasing
	}

	gatheredChunkCerts := n.storage.GatherChunkCerts()
	emapChunkCerts := make([]*emapChunkCertificate, len(gatheredChunkCerts))
	for i := range emapChunkCerts {
		emapChunkCerts[i] = &emapChunkCertificate{*gatheredChunkCerts[i]}
	}
	duplicates, err := n.validityWindow.IsRepeat(ctx, NewValidityWindowBlock(parent), timestamp, emapChunkCerts)
	if err != nil {
		return Block{}, err
	}

	availableChunkCerts := make([]*ChunkCertificate, 0)
	for i, chunkCert := range gatheredChunkCerts {
		// avoid building blocks with duplicate or expired chunk certs
		if chunkCert.Expiry < timestamp || duplicates.Contains(i) {
			continue
		}
		availableChunkCerts = append(availableChunkCerts, chunkCert)
	}
	if len(availableChunkCerts) == 0 {
		return Block{}, ErrNoAvailableChunkCerts
	}

	blk := Block{
		BlockHeader: BlockHeader{
			ParentID:  parent.GetID(),
			Height:    parent.Height + 1,
			Timestamp: timestamp,
		},
		ChunkCerts: availableChunkCerts,
	}

	packer := wrappers.Packer{Bytes: make([]byte, 0, InitialChunkSize), MaxSize: consts.NetworkSizeLimit}
	if err := codec.LinearCodec.MarshalInto(blk, &packer); err != nil {
		return Block{}, err
	}

	blk.blkBytes = packer.Bytes
	blk.blkID = utils.ToID(blk.blkBytes)
	return blk, nil
}

func (n *Node[T]) Verify(ctx context.Context, parent Block, block Block) error {
	if block.ParentID != parent.GetID() {
		return fmt.Errorf(
			"%w %s: expected %s",
			ErrInvalidBlockParent,
			block.ParentID,
			parent.GetID(),
		)
	}

	if block.Height != parent.Height+1 {
		return fmt.Errorf(
			"%w %d: expected %d",
			ErrInvalidBlockHeight,
			block.Height,
			parent.Height+1,
		)
	}

	if block.Timestamp <= parent.Timestamp ||
		block.Timestamp > parent.Timestamp+maxTimeSkew.Nanoseconds() {
		return fmt.Errorf("%w %d: parent - %d", ErrInvalidBlockTimestamp, block.Timestamp, parent.Timestamp)
	}

	if len(block.ChunkCerts) == 0 {
		return fmt.Errorf("%w: %s", ErrEmptyBlock, block.GetID())
	}

	// Find repeats
	if err := n.validityWindow.VerifyExpiryReplayProtection(ctx, NewValidityWindowBlock(block)); err != nil {
		return err
	}

	for _, chunkCert := range block.ChunkCerts {
		if err := chunkCert.Verify(
			ctx,
			n.chainState,
		); err != nil {
			return fmt.Errorf("%w %s: %w", ErrInvalidWarpSignature, chunkCert.ChunkID, err)
		}
	}

	return nil
}

func (n *Node[T]) Accept(ctx context.Context, block Block) (ExecutedBlock[T], error) {
	acceptedChunkIDs := make([]ids.ID, 0, len(block.ChunkCerts))
	chunks := make([]Chunk[T], 0, len(block.ChunkCerts))

	for _, chunkCert := range block.ChunkCerts {
		acceptedChunkIDs = append(acceptedChunkIDs, chunkCert.ChunkID)

		chunkBytes, err := n.storage.GetChunkBytes(chunkCert.Expiry, chunkCert.ChunkID)
		if errors.Is(err, database.ErrNotFound) {
			for {
				result := make(chan error)
				onResponse := func(_ context.Context, _ ids.NodeID, response Chunk[T], err error) {
					defer close(result)
					if err != nil {
						result <- err
						return
					}

					if _, err := n.storage.VerifyRemoteChunk(response); err != nil {
						result <- err
						return
					}

					chunks = append(chunks, response)
				}

				// TODO better request strategy
				validators, err := n.chainState.GetCanonicalValidatorSet(ctx)
				if err != nil {
					return ExecutedBlock[T]{}, fmt.Errorf("failed to retrieve validators list: %w", err)
				}
				nodeID := validators.Validators[rand.Intn(len(validators.Validators))].NodeIDs[0] //nolint:gosec
				if err := n.getChunkClient.AppRequest(
					ctx,
					nodeID,
					&dsmr.GetChunkRequest{
						ChunkId: chunkCert.ChunkID[:],
						Expiry:  chunkCert.Expiry,
					},
					onResponse,
				); err != nil {
					return ExecutedBlock[T]{}, fmt.Errorf("failed to request chunk referenced in block: %w", err)
				}

				if <-result == nil {
					break
				}
			}
		}

		chunk, err := ParseChunk[T](chunkBytes)
		if err != nil {
			return ExecutedBlock[T]{}, fmt.Errorf("failed to parse chunk: %w", err)
		}

		chunks = append(chunks, chunk)
	}
	// update the validity window with the accepted block.
	n.validityWindow.Accept(NewValidityWindowBlock(block))

	if err := n.storage.SetMin(block.Timestamp, acceptedChunkIDs); err != nil {
		return ExecutedBlock[T]{}, fmt.Errorf("failed to prune chunks: %w", err)
	}

	n.LastAccepted = block
	return ExecutedBlock[T]{
		BlockHeader: BlockHeader{
			ParentID:  block.ParentID,
			Height:    block.Height,
			Timestamp: block.Timestamp,
		},
		ID:     block.GetID(),
		Chunks: chunks,
	}, nil
}

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
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
	"github.com/ava-labs/hypersdk/utils"

	snowValidators "github.com/ava-labs/avalanchego/snow/validators"
)

const (
	maxTimeSkew = 30 * time.Second
)

var (
	_ snowValidators.State = (*pChain)(nil)

	ErrEmptyChunk                          = errors.New("empty chunk")
	ErrNoAvailableChunkCerts               = errors.New("no available chunk certs")
	ErrAllChunkCertsDuplicate              = errors.New("all chunk certs are duplicated")
	ErrTimestampNotMonotonicallyIncreasing = errors.New("block timestamp must be greater than parent timestamp")
	ErrEmptyBlock                          = errors.New("block must reference chunks")
	ErrInvalidBlockParent                  = errors.New("invalid referenced block parent")
	ErrInvalidBlockHeight                  = errors.New("invalid block height")
	ErrInvalidBlockTimestamp               = errors.New("invalid block timestamp")
	ErrInvalidWarpSignature                = errors.New("invalid warp signature")
	ErrInvalidSignatureType                = errors.New("invalid signature type")
	ErrFailedToReplicate                   = errors.New("failed to replicate to sufficient stake")
)

type Validator struct {
	NodeID    ids.NodeID
	Weight    uint64
	PublicKey *bls.PublicKey
}

func New[T Tx](
	log logging.Logger,
	tracer trace.Tracer,
	nodeID ids.NodeID,
	networkID uint32,
	chainID ids.ID,
	pk *bls.PublicKey,
	signer warp.Signer,
	chunkStorage *ChunkStorage[T],
	getChunkHandler p2p.Handler,
	getChunkSignatureHandler p2p.Handler,
	chunkCertificateGossipHandler p2p.Handler,
	getChunkClient *p2p.Client,
	getChunkSignatureClient *p2p.Client,
	chunkCertificateGossipClient *p2p.Client,
	validators []Validator, // TODO remove hard-coded validator set
	lastAccepted Block,
	quorumNum uint64,
	quorumDen uint64,
	chainIndex validitywindow.ChainIndex[*ChunkCertificate],
	validityWindowDuration time.Duration,
) (*Node[T], error) {
	return &Node[T]{
		ID:                            nodeID,
		LastAccepted:                  lastAccepted,
		networkID:                     networkID,
		chainID:                       chainID,
		PublicKey:                     pk,
		Signer:                        signer,
		getChunkClient:                NewGetChunkClient[T](getChunkClient),
		chunkCertificateGossipClient:  NewChunkCertificateGossipClient(chunkCertificateGossipClient),
		validators:                    validators,
		quorumNum:                     quorumNum,
		quorumDen:                     quorumDen,
		chunkSignatureAggregator:      acp118.NewSignatureAggregator(log, getChunkSignatureClient),
		GetChunkHandler:               getChunkHandler,
		GetChunkSignatureHandler:      getChunkSignatureHandler,
		ChunkCertificateGossipHandler: chunkCertificateGossipHandler,
		storage:                       chunkStorage,
		log:                           log,
		tracer:                        tracer,
		validityWindow:                validitywindow.NewTimeValidityWindow(log, tracer, chainIndex),
		validityWindowDuration:        validityWindowDuration,
	}, nil
}

type Node[T Tx] struct {
	ID                           ids.NodeID
	PublicKey                    *bls.PublicKey
	Signer                       warp.Signer
	LastAccepted                 Block
	networkID                    uint32
	chainID                      ids.ID
	getChunkClient               *TypedClient[*dsmr.GetChunkRequest, Chunk[T], []byte]
	chunkCertificateGossipClient *TypedClient[[]byte, []byte, *dsmr.ChunkCertificateGossip]
	validators                   []Validator
	quorumNum                    uint64
	quorumDen                    uint64
	chunkSignatureAggregator     *acp118.SignatureAggregator

	GetChunkHandler               p2p.Handler
	GetChunkSignatureHandler      p2p.Handler
	ChunkCertificateGossipHandler p2p.Handler
	storage                       *ChunkStorage[T]
	log                           logging.Logger
	tracer                        trace.Tracer
	validityWindowDuration        time.Duration
	validityWindow                *validitywindow.TimeValidityWindow[*ChunkCertificate]
}

// BuildChunk builds transactions into a Chunk
// TODO handle frozen sponsor + validator assignments
func (n *Node[T]) BuildChunk(
	ctx context.Context,
	txs []T,
	expiry int64,
	beneficiary codec.Address,
) (Chunk[T], ChunkCertificate, error) {
	if len(txs) == 0 {
		return Chunk[T]{}, ChunkCertificate{}, ErrEmptyChunk
	}

	chunk, err := signChunk[T](
		UnsignedChunk[T]{
			Producer:    n.ID,
			Beneficiary: beneficiary,
			Expiry:      expiry,
			Txs:         txs,
		},
		n.networkID,
		n.chainID,
		n.PublicKey,
		n.Signer,
	)
	if err != nil {
		return Chunk[T]{}, ChunkCertificate{}, fmt.Errorf("failed to sign chunk: %w", err)
	}

	packer := wrappers.Packer{MaxSize: MaxMessageSize}
	if err := codec.LinearCodec.MarshalInto(ChunkReference{
		ChunkID:  chunk.id,
		Producer: chunk.Producer,
		Expiry:   chunk.Expiry,
	}, &packer); err != nil {
		return Chunk[T]{}, ChunkCertificate{}, fmt.Errorf("failed to marshal chunk reference: %w", err)
	}

	unsignedMsg, err := warp.NewUnsignedMessage(n.networkID, n.chainID, packer.Bytes)
	if err != nil {
		return Chunk[T]{}, ChunkCertificate{}, fmt.Errorf("failed to initialize unsigned warp message: %w", err)
	}
	msg, err := warp.NewMessage(
		unsignedMsg,
		&warp.BitSetSignature{
			Signature: [bls.SignatureLen]byte{},
		},
	)
	if err != nil {
		return Chunk[T]{}, ChunkCertificate{}, fmt.Errorf("failed to initialize warp message: %w", err)
	}

	canonicalValidators, _, err := warp.GetCanonicalValidatorSet(
		ctx,
		pChain{validators: n.validators},
		0,
		ids.Empty,
	)
	if err != nil {
		return Chunk[T]{}, ChunkCertificate{}, fmt.Errorf("failed to get canonical validator set: %w", err)
	}

	aggregatedMsg, _, _, ok, err := n.chunkSignatureAggregator.AggregateSignatures(
		ctx,
		msg,
		chunk.bytes,
		canonicalValidators,
		n.quorumNum,
		n.quorumDen,
	)
	if err != nil {
		return Chunk[T]{}, ChunkCertificate{}, fmt.Errorf("failed to aggregate signatures: %w", err)
	}

	if !ok {
		return Chunk[T]{}, ChunkCertificate{}, ErrFailedToReplicate
	}

	bitSetSignature, ok := aggregatedMsg.Signature.(*warp.BitSetSignature)
	if !ok {
		return Chunk[T]{}, ChunkCertificate{}, ErrInvalidSignatureType
	}

	chunkCert := ChunkCertificate{
		ChunkReference: ChunkReference{
			ChunkID:  chunk.id,
			Producer: chunk.Producer,
			Expiry:   chunk.Expiry,
		},
		Signature: bitSetSignature,
	}

	packer = wrappers.Packer{MaxSize: MaxMessageSize}
	if err := codec.LinearCodec.MarshalInto(&chunkCert, &packer); err != nil {
		return Chunk[T]{}, ChunkCertificate{}, err
	}

	if err := n.chunkCertificateGossipClient.AppGossip(
		ctx,
		&dsmr.ChunkCertificateGossip{ChunkCertificate: packer.Bytes},
	); err != nil {
		return Chunk[T]{}, ChunkCertificate{}, err
	}

	return chunk, chunkCert, n.storage.AddLocalChunkWithCert(chunk, &chunkCert)
}

func (n *Node[T]) BuildBlock(ctx context.Context, parent Block, timestamp int64) (Block, error) {
	if timestamp <= parent.Tmstmp {
		return Block{}, ErrTimestampNotMonotonicallyIncreasing
	}

	chunkCerts := n.storage.GatherChunkCerts()
	oldestAllowed := max(0, timestamp-int64(n.validityWindowDuration))

	dup, err := n.validityWindow.IsRepeat(ctx, parent, chunkCerts, oldestAllowed)
	if err != nil {
		return Block{}, err
	}

	availableChunkCerts := make([]*ChunkCertificate, 0)
	for i, chunkCert := range chunkCerts {
		// avoid building blocks with duplicate or expired chunk certs
		if chunkCert.Expiry < timestamp || dup.Contains(i) {
			continue
		}
		availableChunkCerts = append(availableChunkCerts, chunkCert)
	}
	if len(availableChunkCerts) == 0 {
		return Block{}, ErrNoAvailableChunkCerts
	}

	blk := NewBlock(
		parent.GetID(),
		parent.Hght+1,
		timestamp,
		availableChunkCerts,
	)

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

	if block.Hght != parent.Hght+1 {
		return fmt.Errorf(
			"%w %d: expected %d",
			ErrInvalidBlockHeight,
			block.Hght,
			parent.Hght+1,
		)
	}

	if block.Tmstmp <= parent.Tmstmp ||
		block.Tmstmp > parent.Tmstmp+maxTimeSkew.Nanoseconds() {
		return fmt.Errorf("%w %d: parent - %d", ErrInvalidBlockTimestamp, block.Tmstmp, parent.Tmstmp)
	}

	if len(block.ChunkCerts) == 0 {
		return fmt.Errorf("%w: %s", ErrEmptyBlock, block.GetID())
	}

	// Find repeats
	oldestAllowed := max(0, block.Timestamp()-int64(n.validityWindowDuration))

	if err := n.validityWindow.VerifyExpiryReplayProtection(ctx, block, oldestAllowed); err != nil {
		return err
	}

	for _, chunkCert := range block.ChunkCerts {
		if err := chunkCert.Verify(
			ctx,
			n.networkID,
			n.chainID,
			pChain{validators: n.validators},
			0,
			n.quorumNum,
			n.quorumDen,
		); err != nil {
			return fmt.Errorf("%w %s: %w", ErrInvalidWarpSignature, chunkCert.ChunkID, err)
		}
	}

	return nil
}

func (n *Node[T]) Accept(ctx context.Context, block Block) error {
	chunkIDs := make([]ids.ID, 0, len(block.ChunkCerts))
	for _, chunkCert := range block.ChunkCerts {
		chunkIDs = append(chunkIDs, chunkCert.ChunkID)

		_, _, err := n.storage.GetChunkBytes(chunkCert.Expiry, chunkCert.ChunkID)
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
				}

				// TODO better request strategy
				nodeID := n.validators[rand.Intn(len(n.validators))].NodeID //nolint:gosec
				if err := n.getChunkClient.AppRequest(
					ctx,
					nodeID,
					&dsmr.GetChunkRequest{
						ChunkId: chunkCert.ChunkID[:],
						Expiry:  chunkCert.Expiry,
					},
					onResponse,
				); err != nil {
					return fmt.Errorf("failed to request chunk referenced in block: %w", err)
				}

				if <-result == nil {
					break
				}
			}
		}
	}
	// update the validity window with the accepted block.
	n.validityWindow.Accept(block)

	if err := n.storage.SetMin(block.Tmstmp, chunkIDs); err != nil {
		return fmt.Errorf("failed to prune chunks: %w", err)
	}

	n.LastAccepted = block
	return nil
}

type pChain struct {
	validators []Validator
}

func (pChain) GetMinimumHeight(context.Context) (uint64, error) {
	return 0, nil
}

func (pChain) GetCurrentHeight(context.Context) (uint64, error) {
	return 0, nil
}

func (pChain) GetSubnetID(context.Context, ids.ID) (ids.ID, error) {
	return ids.Empty, nil
}

func (p pChain) GetValidatorSet(context.Context, uint64, ids.ID) (map[ids.NodeID]*snowValidators.GetValidatorOutput, error) {
	result := make(map[ids.NodeID]*snowValidators.GetValidatorOutput, len(p.validators))
	for _, v := range p.validators {
		result[v.NodeID] = &snowValidators.GetValidatorOutput{
			NodeID:    v.NodeID,
			PublicKey: v.PublicKey,
			Weight:    v.Weight,
		}
	}

	return result, nil
}

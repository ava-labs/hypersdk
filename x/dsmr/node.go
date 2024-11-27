// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/acp118"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
	"github.com/ava-labs/hypersdk/utils"

	snowValidators "github.com/ava-labs/avalanchego/snow/validators"
)

var (
	_ snowValidators.State = (*pChain)(nil)

	ErrEmptyChunk                          = errors.New("empty chunk")
	ErrNoAvailableChunkCerts               = errors.New("no available chunk certs")
	ErrTimestampNotMonotonicallyIncreasing = errors.New("block timestamp must be greater than parent timestamp")
)

type Validator struct {
	NodeID    ids.NodeID
	Weight    uint64
	PublicKey *bls.PublicKey
}

func New[T Tx](
	log logging.Logger,
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
	quorumNum uint64,
	quorumDen uint64,
) (*Node[T], error) {
	return &Node[T]{
		ID:                            nodeID,
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
	}, nil
}

type Node[T Tx] struct {
	ID                           ids.NodeID
	PublicKey                    *bls.PublicKey
	Signer                       warp.Signer
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
}

// BuildChunk builds transactions into a Chunk
// TODO handle frozen sponsor + validator assignments
func (n *Node[T]) BuildChunk(
	ctx context.Context,
	txs []T,
	expiry int64,
	beneficiary codec.Address,
) (Chunk[T], error) {
	if len(txs) == 0 {
		return Chunk[T]{}, ErrEmptyChunk
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
		return Chunk[T]{}, fmt.Errorf("failed to sign chunk: %w", err)
	}

	packer := wrappers.Packer{MaxSize: MaxMessageSize}
	if err := codec.LinearCodec.MarshalInto(chunk, &packer); err != nil {
		return Chunk[T]{}, err
	}

	unsignedMsg, err := warp.NewUnsignedMessage(n.networkID, n.chainID, packer.Bytes)
	if err != nil {
		return Chunk[T]{}, fmt.Errorf("failed to initialize unsigned warp message: %w", err)
	}
	msg, err := warp.NewMessage(
		unsignedMsg,
		&warp.BitSetSignature{
			Signature: [bls.SignatureLen]byte{},
		},
	)
	if err != nil {
		return Chunk[T]{}, fmt.Errorf("failed to initialize warp message: %w", err)
	}

	canonicalValidators, _, err := warp.GetCanonicalValidatorSet(
		ctx,
		pChain{validators: n.validators},
		0,
		ids.Empty,
	)
	if err != nil {
		return Chunk[T]{}, fmt.Errorf("failed to get canonical validator set: %w", err)
	}

	aggregatedMsg, _, _, ok, err := n.chunkSignatureAggregator.AggregateSignatures(
		ctx,
		msg,
		nil, // justification is unused and is safe to be nil
		canonicalValidators,
		n.quorumNum,
		n.quorumDen,
	)
	if err != nil {
		return Chunk[T]{}, fmt.Errorf("failed to aggregate signatures: %w", err)
	}

	if !ok {
		return Chunk[T]{}, errors.New("failed to replicate to sufficient stake")
	}

	bitSetSignature, ok := aggregatedMsg.Signature.(*warp.BitSetSignature)
	if !ok {
		return Chunk[T]{}, errors.New("invalid signature type")
	}

	chunkCert := ChunkCertificate[T]{
		ChunkID:   chunk.id,
		Expiry:    chunk.Expiry,
		Signature: bitSetSignature,
	}

	packer = wrappers.Packer{MaxSize: MaxMessageSize}
	if err := codec.LinearCodec.MarshalInto(&chunkCert, &packer); err != nil {
		return Chunk[T]{}, err
	}

	if err := n.chunkCertificateGossipClient.AppGossip(
		ctx,
		&dsmr.ChunkCertificateGossip{ChunkCertificate: packer.Bytes},
	); err != nil {
		return Chunk[T]{}, err
	}

	return chunk, n.storage.AddLocalChunkWithCert(chunk, &chunkCert)
}

func (n *Node[T]) BuildBlock(parent Block[T], timestamp int64) (Block[T], error) {
	if timestamp <= parent.Timestamp {
		return Block[T]{}, ErrTimestampNotMonotonicallyIncreasing
	}

	chunkCerts := n.storage.GatherChunkCerts()
	availableChunkCerts := make([]*ChunkCertificate[T], 0)
	for _, chunkCert := range chunkCerts {
		// avoid building blocks with expired chunk certs
		if chunkCert.Expiry < timestamp {
			continue
		}

		availableChunkCerts = append(availableChunkCerts, chunkCert)
	}
	if len(availableChunkCerts) == 0 {
		return Block[T]{}, ErrNoAvailableChunkCerts
	}

	blk := Block[T]{
		ParentID:   parent.GetID(),
		Height:     parent.Height + 1,
		Timestamp:  timestamp,
		ChunkCerts: availableChunkCerts,
	}

	packer := wrappers.Packer{Bytes: make([]byte, 0, InitialChunkSize), MaxSize: consts.NetworkSizeLimit}
	if err := codec.LinearCodec.MarshalInto(blk, &packer); err != nil {
		return Block[T]{}, err
	}

	blk.blkBytes = packer.Bytes
	blk.blkID = utils.ToID(blk.blkBytes)
	return blk, nil
}

func (n *Node[T]) Execute(ctx context.Context, block Block[T]) error {
	// TODO: Verify header fields
	// TODO: de-duplicate chunk certificates (internal to block and across history)
	for _, chunkCert := range block.ChunkCerts {
		if err := chunkCert.Verify(
			ctx,
			n.storage,
			n.networkID,
			n.chainID,
			pChain{validators: n.validators},
			0,
			n.quorumNum,
			n.quorumDen,
		); err != nil {
			return err
		}
	}

	return nil
}

func (n *Node[T]) Accept(ctx context.Context, block Block[T]) error {
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

	return n.storage.SetMin(block.Timestamp, chunkIDs)
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
	result := make(map[ids.NodeID]*snowValidators.GetValidatorOutput)
	for _, v := range p.validators {
		result[v.NodeID] = &snowValidators.GetValidatorOutput{
			NodeID:    v.NodeID,
			PublicKey: v.PublicKey,
			Weight:    v.Weight,
		}
	}

	return result, nil
}

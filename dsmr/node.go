// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	ErrEmptyChunk                          = errors.New("empty chunk")
	ErrNoAvailableChunkCerts               = errors.New("no available chunk certs")
	ErrTimestampNotMonotonicallyIncreasing = errors.New("block timestamp must be greater than parent timestamp")
)

type Validator struct {
	NodeID ids.NodeID
}

func New[T Tx](
	nodeID ids.NodeID,
	sk *bls.SecretKey,
	chunkVerifier Verifier[T],
	getChunkClient *p2p.Client,
	getChunkSignatureClient *p2p.Client,
	chunkCertificateGossipClient *p2p.Client,
	validators []Validator,
) (*Node[T], error) {
	storage, err := newChunkStorage[T](NoVerifier[T]{}, memdb.New())
	if err != nil {
		return nil, err
	}

	return &Node[T]{
		nodeID:                       nodeID,
		sk:                           sk,
		getChunkClient:               NewGetChunkClient(getChunkClient),
		getChunkSignatureClient:      NewGetChunkSignatureClient(getChunkSignatureClient),
		chunkCertificateGossipClient: NewChunkCertificateGossipClient(chunkCertificateGossipClient),
		validators:                   validators,
		GetChunkHandler: &GetChunkHandler[T]{
			storage: storage,
		},
		GetChunkSignatureHandler: &GetChunkSignatureHandler[T]{
			sk:       sk,
			verifier: chunkVerifier,
			storage:  storage,
		},
		ChunkCertificateGossipHandler: &ChunkCertificateGossipHandler[T]{
			storage: storage,
		},
		storage: storage,
		pk:      bls.PublicFromSecretKey(sk),
	}, nil
}

type Node[T Tx] struct {
	nodeID                       ids.NodeID
	sk                           *bls.SecretKey
	networkID                    uint32
	chainID                      ids.ID
	getChunkClient               *TypedClient[*dsmr.GetChunkRequest, *dsmr.GetChunkResponse, []byte]
	getChunkSignatureClient      *TypedClient[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse, []byte]
	chunkCertificateGossipClient *TypedClient[[]byte, []byte, *dsmr.ChunkCertificateGossip]
	validators                   []Validator

	GetChunkHandler               *GetChunkHandler[T]
	GetChunkSignatureHandler      *GetChunkSignatureHandler[T]
	ChunkCertificateGossipHandler *ChunkCertificateGossipHandler[T]
	//TODO chunk handler
	storage *chunkStorage[T]
	pk      *bls.PublicKey
}

// NewChunk builds transactions into a Chunk
// TODO handle frozen sponsor + validator assignments
func (n *Node[T]) NewChunk(
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
			Producer:    n.nodeID,
			Beneficiary: beneficiary,
			Expiry:      expiry,
			Txs:         txs,
		},
		n.sk,
		n.pk,
		n.networkID,
		n.chainID,
	)
	if err != nil {
		return Chunk[T]{}, fmt.Errorf("failed to sign chunk: %w", err)
	}

	protoChunk, err := newProtoChunk[T](chunk)
	if err != nil {
		return Chunk[T]{}, fmt.Errorf("failed to marshal chunk: %w", err)
	}

	request := &dsmr.GetChunkSignatureRequest{
		Chunk: protoChunk,
	}

	for _, validator := range n.validators {
		done := make(chan struct{})
		onResponse := func(ctx context.Context, nodeID ids.NodeID, response *dsmr.GetChunkSignatureResponse, err error) {
			defer close(done)
			//TODO generate chunk cert
		}

		if err := n.getChunkSignatureClient.AppRequest(ctx, validator.NodeID, request, onResponse); err != nil {
			return Chunk[T]{}, err
		}

		<-done
	}

	chunkCert := &ChunkCertificate{
		ChunkID:   chunk.id,
		Expiry:    chunk.Expiry,
		Signature: NoVerifyChunkSignature{},
	}

	if err := n.chunkCertificateGossipClient.AppGossip(ctx, &dsmr.ChunkCertificateGossip{
		ChunkCertificate: &dsmr.ChunkCertificate{
			ChunkId:   chunk.id[:],
			Producer:  chunk.Producer[:],
			Expiry:    chunk.Expiry,
			Signers:   nil, // TODO populate
			Signature: nil,
		},
	}); err != nil {
		return Chunk[T]{}, err
	}

	return chunk, n.storage.AddLocalChunkWithCert(chunk, chunkCert)
}

func (n *Node[T]) NewBlock(parent Block, timestamp int64) (Block, error) {
	if timestamp <= parent.Timestamp {
		return Block{}, ErrTimestampNotMonotonicallyIncreasing
	}

	chunkCerts := n.storage.GatherChunkCerts()
	availableChunkCerts := make([]*ChunkCertificate, 0)
	for _, chunkCert := range chunkCerts {
		// avoid building blocks with expired chunk certs
		if chunkCert.Expiry < timestamp {
			continue
		}

		availableChunkCerts = append(availableChunkCerts, chunkCert)
	}
	if len(availableChunkCerts) == 0 {
		return Block{}, ErrNoAvailableChunkCerts
	}

	blk := Block{
		ParentID:   parent.GetID(),
		Height:     parent.Height + 1,
		Timestamp:  timestamp,
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

func (n *Node[T]) Accept(ctx context.Context, block Block) error {
	chunkIDs := make([]ids.ID, 0, len(block.ChunkCerts))
	for _, chunkCert := range block.ChunkCerts {
		chunkIDs = append(chunkIDs, chunkCert.ChunkID)

		_, _, err := n.storage.GetChunkBytes(chunkCert.Expiry, chunkCert.ChunkID)
		if errors.Is(err, database.ErrNotFound) {
			for {
				result := make(chan error)
				onResponse := func(ctx context.Context, nodeID ids.NodeID, response *dsmr.GetChunkResponse, err error) {
					defer close(result)
					if err != nil {
						result <- err
						return
					}

					chunk, err := newChunkFromProto[T](response.Chunk)
					if err != nil {
						result <- err
						return
					}

					if _, err := n.storage.VerifyRemoteChunk(chunk); err != nil {
						result <- err
						return
					}
				}

				// TODO better request strategy
				nodeID := n.validators[rand.Intn(len(n.validators))].NodeID
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

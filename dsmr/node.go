// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var ErrEmptyChunk = errors.New("empty chunk")

func New[T Tx](
	nodeID ids.NodeID,
	sk *bls.SecretKey,
	beneficiary codec.Address,
	chunkVerifier Verifier[T],
	getChunkClient *p2p.Client,
) (*Node[T], error) {
	storage, err := newChunkStorage[T](NoVerifier[T]{}, memdb.New())
	if err != nil {
		return nil, err
	}

	return &Node[T]{
		nodeID:      nodeID,
		sk:          sk,
		beneficiary: beneficiary,
		client: NewTypedClient[*dsmr.GetChunkRequest, *dsmr.GetChunkResponse](
			getChunkClient,
			getChunkMarshaler{},
		),
		GetChunkHandler: &GetChunkHandler[T]{
			storage: storage,
		},
		GetChunkSignatureHandler: &GetChunkSignatureHandler[T]{
			sk:       sk,
			verifier: chunkVerifier,
			storage:  storage,
		},
		storage: storage,
		pk:      bls.PublicFromSecretKey(sk),
	}, nil
}

//// TODO this is a hack
//func (n *Node[T]) getValidators() []Validator {
//	rand.Shuffle(len(n.validators), func(i, j int) {
//		n.validators[i], n.validators[j] = n.validators[j], n.validators[i]
//	})
//
//	return n.validators
//}

type Node[T Tx] struct {
	nodeID      ids.NodeID
	sk          *bls.SecretKey
	networkID   uint32
	chainID     ids.ID
	beneficiary codec.Address
	client      *TypedClient[*dsmr.GetChunkRequest, *dsmr.GetChunkResponse]

	GetChunkHandler          *GetChunkHandler[T]
	GetChunkSignatureHandler *GetChunkSignatureHandler[T]
	//TODO chunk handler
	storage *chunkStorage[T]
	pk      *bls.PublicKey
}

// NewChunk builds transactions into a Chunk
// TODO handle frozen sponsor + validator assignments
func (n *Node[T]) NewChunk(txs []T, expiry time.Time) (Chunk[T], error) {
	if len(txs) == 0 {
		return Chunk[T]{}, ErrEmptyChunk
	}

	chunk, err := signChunk[T](
		UnsignedChunk[T]{
			Producer:    n.nodeID,
			Beneficiary: n.beneficiary,
			Expiry:      expiry.Unix(),
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

	//TODO make chunk certs
	chunkCert := &ChunkCertificate{
		ChunkID:   chunk.id,
		Expiry:    chunk.Expiry,
		Signature: NoVerifyChunkSignature{},
	}

	//TODO gossip chunk certs
	return chunk, n.storage.AddLocalChunkWithCert(chunk, chunkCert)
}

// NewBlock TODO should we quiesce
func (n *Node[T]) NewBlock() Block {
	return Block{
		ChunkCerts: n.storage.GatherChunkCerts(),
	}
}

func (n *Node[T]) Accept(ctx context.Context, block Block) error {
	expiry := int64(0)
	chunkIDs := make([]ids.ID, 0, len(block.ChunkCerts))
	for _, chunkCert := range block.ChunkCerts {
		expiry = max(expiry, chunkCert.Expiry)
		chunkIDs = append(chunkIDs, chunkCert.ChunkID)

		_, _, err := n.storage.GetChunkBytes(chunkCert.Expiry, chunkCert.ChunkID)
		if errors.Is(err, database.ErrNotFound) {
			onResponse := func(ctx context.Context, nodeID ids.NodeID, response *dsmr.GetChunkResponse, err error) {

			}
			// TODO which validators do we request from?
			if err := n.client.AppRequest(
				ctx,
				set.Of(ids.EmptyNodeID),
				&dsmr.GetChunkRequest{
					ChunkId: chunkCert.ChunkID[:],
					Expiry:  chunkCert.Expiry,
				},
				onResponse,
			); err != nil {
				return fmt.Errorf("failed to request chunk referenced in block: %w", err)
			}
		}
	}

	return n.storage.SetMin(expiry, chunkIDs)
}

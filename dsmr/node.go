// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"time"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/hypersdk/codec"
)

func New[T Tx](
	nodeID ids.NodeID,
	sk *bls.SecretKey,
	beneficiary codec.Address,
	getChunkClient *p2p.Client,
) (*Node[T], error) {
	storage, err := newChunkStorage[T](&NoVerifier[T]{}, memdb.New())
	if err != nil {
		return nil, err
	}

	return &Node[T]{
		nodeID:      nodeID,
		sk:          sk,
		pk:          bls.PublicFromSecretKey(sk),
		beneficiary: beneficiary,
		GetChunkHandler: &GetChunkHandler[T]{
			storage: storage,
		},
		GetChunkSignatureHandler: &GetChunkSignatureHandler[T]{
			storage: storage,
		},
		client: GetChunkClient[T]{
			client: getChunkClient,
		},
		storage: storage,
	}, nil
}

type Node[T Tx] struct {
	//TODO cleanup struct
	nodeID                   ids.NodeID
	sk                       *bls.SecretKey
	pk                       *bls.PublicKey
	beneficiary              codec.Address
	GetChunkHandler          *GetChunkHandler[T]
	GetChunkSignatureHandler *GetChunkSignatureHandler[T]

	//TODO chunk handler
	client           GetChunkClient[T]
	chunkCertBuilder chunkCertBuilder[T]
	storage          *chunkStorage[T]
}

// BuildChunk adds a chunk to the node with the provided transactions
// TODO why return error
// TODO handle frozen sponsor + validator assignments
func (n Node[T]) BuildChunk(txs []T, expiry time.Time) (Chunk[T], error) {
	// TODO aggregate signatures + gossip
	chunk, err := newChunk[T](
		n.nodeID,
		txs,
		expiry,
		n.beneficiary,
		[48]byte{},
		[96]byte{},
	)
	if err != nil {
		return Chunk[T]{}, nil
	}

	cert := &ChunkCertificate{
		ChunkID:   chunk.id,
		Expiry:    chunk.Expiry,
		Signature: NoVerifyChunkSignature{},
	}

	return chunk, n.storage.AddLocalChunkWithCert(chunk, cert)
}

// NewBlock TODO should we quiesce
func (n Node[T]) NewBlock() (Block, error) {
	return Block{
		Chunks: n.storage.GatherChunkCerts(),
	}, nil
}

// consumes chunks and aggregates signtures to generate chunk certs
type chunkCertBuilder[T Tx] struct {
	client *p2p.Client
}

// TODO implement
func (c *chunkCertBuilder[T]) NewCert(chunk Chunk[T]) (ChunkCertificate, error) {
	return ChunkCertificate{}, nil
}

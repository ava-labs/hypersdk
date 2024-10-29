// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"time"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/acp118"
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
		ACP118Handler: nil,
		client: GetChunkClient[T]{
			client: getChunkClient,
		},
		storage: storage,
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
	networkID uint32
	chainID   ids.ID

	//validators []acp118.Validator
	//aggregator *acp118.SignatureAggregator

	//TODO cleanup struct
	nodeID                   ids.NodeID
	sk                       *bls.SecretKey
	pk                       *bls.PublicKey
	beneficiary              codec.Address
	GetChunkHandler          *GetChunkHandler[T]
	GetChunkSignatureHandler *GetChunkSignatureHandler[T]
	ACP118Handler            *acp118.Handler

	//TODO chunk handler
	client           GetChunkClient[T]
	acp118Client     *p2p.Client
	chunkCertBuilder chunkCertBuilder[T]
	storage          *chunkStorage[T]
}

// NewChunk builds transactions into a Chunk
// TODO why return error
// TODO handle frozen sponsor + validator assignments
func (n *Node[T]) NewChunk(txs []T, expiry time.Time) (Chunk[T], error) {
	//msg, err := warp.NewUnsignedMessage(n.networkID, n.chainID, chunk.bytes)
	//if err != nil {
	//	return Chunk[T]{}, err
	//}
	//
	//totalWeight := uint64(0)
	//for _, v := range n.validators {
	//	totalWeight += v.Weight
	//}
	//
	//signedMessage, err := n.aggregator.AggregateSignatures(
	//	ctx,
	//	msg,
	//	nil,
	//	n.validators,
	//	totalWeight*2/3+1,
	//)
	//if err != nil {
	//	return Chunk[T]{}, err
	//}
	//
	//codec.LinearCodec.signedMessage.Signature
	chunk, err := newChunk[T](n.nodeID, txs, expiry, n.beneficiary, [48]byte{}, [96]byte{})
	if err != nil {
		return Chunk[T]{}, err
	}

	cert := &ChunkCertificate{
		ChunkID:   chunk.id,
		Expiry:    chunk.Expiry,
		Signature: NoVerifyChunkSignature{},
	}

	//TODO gossip chunk certs

	return chunk, n.storage.AddLocalChunkWithCert(chunk, cert)
}

// NewBlock TODO should we quiesce
func (n *Node[T]) NewBlock() (Block, error) {
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

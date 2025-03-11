// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/acp118"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
	"go.uber.org/zap"
)

const retryDelay = 50 * time.Millisecond // TODO: add and use util function for exponential backoff

var (
	errReceivedUnexpectedChunk = errors.New("received chunk with unexpected ID")

	_ P2P = (*appP2PClient)(nil)
)

// P2PBackend defines the interface the p2p layer requires to request DSMR performs certain actions
// All implementations must be fully thread-safe.
type P2PBackend interface {
	CommitChunk(chunk *UnsignedChunk) error
	AddChunkCert(cert *ChunkCertificate)
}

// P2P defines the P2P dependency of DSMR itself.
// This interface is agnostic to AvalancheGo AppHandler specifics allowing us to test
// the DSMR logic in isolation from the AvalancheGo specific p2p implementation.
type P2P interface {
	BroadcastChunkCertificate(ctx context.Context, cert *ChunkCertificate) error
	CollectChunkSignature(ctx context.Context, chunk *UnsignedChunk) (*ChunkCertificate, error)
	GatherChunks(ctx context.Context, missingChunkRefs []ChunkReference) ([]*UnsignedChunk, error)
}

type appP2PClient struct {
	log       logging.Logger
	networkID uint32
	chainID   ids.ID
	network   *p2p.Network

	chainState          ChainState
	unsignedChunkClient *UnsignedChunkClient
	// TODO: guarantee that this client performs a broadcast by setting the SendConfig to specify
	// max validators.
	broadcastChunkCertificateClient *ChunkCertificateGossipClient
	chunkSignatureAggregator        *acp118.SignatureAggregator
}

func newAppP2PClient(
	log logging.Logger,
	networkID uint32,
	chainID ids.ID,
	network *p2p.Network,
	getChunkProtocolID uint64,
	broadcastChunkCertProtocolID uint64,
	getChunkSignatureProtocolID uint64,
) *appP2PClient {
	unsignedChunkClient := NewUnsignedChunkClient(network.NewClient(getChunkProtocolID))
	broadcastChunkCertificateClient := NewChunkCertificateGossipClient(network.NewClient(broadcastChunkCertProtocolID))
	acp118Client := network.NewClient(getChunkSignatureProtocolID)

	return &appP2PClient{
		log:                             log,
		networkID:                       networkID,
		chainID:                         chainID,
		network:                         network,
		unsignedChunkClient:             unsignedChunkClient,
		broadcastChunkCertificateClient: broadcastChunkCertificateClient,
		chunkSignatureAggregator: acp118.NewSignatureAggregator(
			log,
			acp118Client,
		),
	}
}

func (c *appP2PClient) BroadcastChunkCertificate(ctx context.Context, cert *ChunkCertificate) error {
	return c.broadcastChunkCertificateClient.AppGossip(
		ctx,
		&dsmr.ChunkCertificateGossip{
			ChunkCertificate: cert.bytes,
		},
	)
}

func (c *appP2PClient) CollectChunkSignature(ctx context.Context, chunk *UnsignedChunk) (*ChunkCertificate, error) {
	chunkRef := chunk.Reference()
	unsignedWarpMsg, err := warp.NewUnsignedMessage(c.networkID, c.chainID, chunkRef.bytes)
	if err != nil {
		return nil, err
	}
	warpMsg, err := warp.NewMessage(unsignedWarpMsg, &warp.BitSetSignature{})
	if err != nil {
		return nil, err
	}
	canonicalVdrSet, err := c.chainState.GetCanonicalValidatorSet(ctx)
	if err != nil {
		return nil, err
	}
	aggregatedMsg, _, _, _, err := c.chunkSignatureAggregator.AggregateSignatures(
		ctx,
		warpMsg,
		chunk.bytes,
		canonicalVdrSet.Validators,
		c.chainState.GetQuorumNum(),
		c.chainState.GetQuorumDen(),
	)
	if err != nil {
		return nil, err
	}
	chunkCert, err := NewChunkCertificate(chunkRef, aggregatedMsg.Signature.(*warp.BitSetSignature))
	if err != nil {
		return nil, fmt.Errorf("failed to construct chunk certificate: %w", err)
	}
	return chunkCert, nil
}

func (c *appP2PClient) GatherChunks(ctx context.Context, missingChunkRefs []ChunkReference) ([]*UnsignedChunk, error) {
	wg := sync.WaitGroup{}
	wg.Add(len(missingChunkRefs))

	chunks := make([]*UnsignedChunk, len(missingChunkRefs))

	// TODO: switch to sync client + consider performing concurrent requests at cost of wasted bandwidth
	for i, chunkRef := range missingChunkRefs {
		go func() {
			defer wg.Done()

			attempt := 0
			for {
				attempt++
				var (
					done       = make(chan struct{})
					foundChunk *UnsignedChunk
					err        error
				)

				nodeID := c.chainState.SampleNodeID()
				if err := c.unsignedChunkClient.AppRequest(ctx, nodeID, &dsmr.GetChunkRequest{
					ChunkId: chunkRef.ChunkID[:],
					Expiry:  chunkRef.Expiry,
				}, func(ctx context.Context, _ ids.NodeID, responseChunk *UnsignedChunk, responseErr error) {
					defer close(done)

					if responseErr != nil {
						err = responseErr
						return
					}

					if responseChunk.id != chunkRef.ChunkID {
						err = fmt.Errorf("%w: %s, expected %s", responseChunk.id, chunkRef.ChunkID)
						return
					}

					foundChunk = responseChunk
				}); err != nil {
					err = err
					close(done)
				}

				<-done
				if err != nil {
					c.log.Debug("failed to fetch chunk",
						zap.Stringer("chunkRef", chunkRef),
						zap.Stringer("nodeID", nodeID),
						zap.Error(err),
					)
					time.Sleep(retryDelay)
					continue
				}
				chunks[i] = foundChunk
				break
			}
		}()
	}
	wg.Wait()

	return chunks, nil
}

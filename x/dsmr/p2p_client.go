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

	_ Client = (*AppClient)(nil)
)

// Client defines the client network dependency of DSMR.
// All implementations must be thread-safe.
type Client interface {
	BroadcastChunkCertificate(ctx context.Context, cert *ChunkCertificate) error
	CollectChunkSignature(ctx context.Context, chunk *Chunk) (*ChunkCertificate, error)
	GatherChunks(ctx context.Context, missingChunkRefs []*ChunkReference) ([]*Chunk, error)
}

type AppClient struct {
	log         logging.Logger
	ruleFactory RuleFactory
	network     *p2p.Network

	chainState          ChainState
	unsignedChunkClient *UnsignedChunkClient
	// TODO: guarantee that this client performs a broadcast by setting the SendConfig to specify
	// max validators.
	broadcastChunkCertificateClient *ChunkCertificateGossipClient
	chunkSignatureAggregator        *acp118.SignatureAggregator
}

func NewAppClient(
	log logging.Logger,
	ruleFactory RuleFactory,
	network *p2p.Network,
	getChunkProtocolID uint64,
	broadcastChunkCertProtocolID uint64,
	getChunkSignatureProtocolID uint64,
) *AppClient {
	unsignedChunkClient := NewChunkClient(network.NewClient(getChunkProtocolID))
	broadcastChunkCertificateClient := NewChunkCertificateGossipClient(network.NewClient(broadcastChunkCertProtocolID))
	acp118Client := network.NewClient(getChunkSignatureProtocolID)

	return &AppClient{
		log:                             log,
		network:                         network,
		unsignedChunkClient:             unsignedChunkClient,
		broadcastChunkCertificateClient: broadcastChunkCertificateClient,
		chunkSignatureAggregator: acp118.NewSignatureAggregator(
			log,
			acp118Client,
		),
	}
}

func (c *AppClient) BroadcastChunkCertificate(ctx context.Context, cert *ChunkCertificate) error {
	c.log.Info("Broadcasting chunk certificate",
		zap.Stringer("chunkRef", cert.Reference),
	)
	return c.broadcastChunkCertificateClient.AppGossip(
		ctx,
		&dsmr.ChunkCertificateGossip{
			ChunkCertificate: cert.bytes,
		},
	)
}

func (c *AppClient) CollectChunkSignature(ctx context.Context, chunk *Chunk) (*ChunkCertificate, error) {
	chunkRef := chunk.CreateReference()
	estimatedPChainHeight, err := c.chainState.EstimatePChainHeight(ctx)
	if err != nil {
		return nil, err
	}
	c.log.Info("Collecting signatures for chunk",
		zap.Stringer("chunkRef", chunkRef),
		zap.Uint64("estimatedPChainHeight", estimatedPChainHeight),
	)
	rules := c.ruleFactory.GetRules(chunk.Expiry)
	unsignedWarpMsg, err := warp.NewUnsignedMessage(rules.NetworkID, rules.ChainID, chunkRef.bytes)
	if err != nil {
		return nil, err
	}
	warpMsg, err := warp.NewMessage(unsignedWarpMsg, &warp.BitSetSignature{})
	if err != nil {
		return nil, err
	}
	canonicalVdrSet, err := c.chainState.GetCanonicalValidatorSet(ctx, estimatedPChainHeight)
	if err != nil {
		return nil, err
	}
	aggregatedMsg, _, _, _, err := c.chunkSignatureAggregator.AggregateSignatures(
		ctx,
		warpMsg,
		chunk.bytes,
		canonicalVdrSet.Validators,
		rules.QuorumNum,
		rules.QuorumDen,
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

func (c *AppClient) GatherChunks(ctx context.Context, missingChunkRefs []*ChunkReference) ([]*Chunk, error) {
	c.log.Info("Gathering missing chunks",
		zap.Stringers("missingChunkRefs", missingChunkRefs),
	)
	wg := sync.WaitGroup{}
	wg.Add(len(missingChunkRefs))

	chunks := make([]*Chunk, len(missingChunkRefs))

	// TODO: switch to sync client + consider performing concurrent requests at cost of wasted bandwidth
	for i, chunkRef := range missingChunkRefs {
		go func() {
			defer wg.Done()

			attempt := 0
			for {
				attempt++
				var (
					done       = make(chan struct{})
					foundChunk *Chunk
					err        error
				)

				// If there are no validators, we can't fetch the chunk. Return a fatal error.
				nodeID, sampleErr := c.chainState.SampleNodeID(ctx)
				if sampleErr != nil {
					err = sampleErr
					close(done)
				}
				if err := c.unsignedChunkClient.AppRequest(ctx, nodeID, &dsmr.GetChunkRequest{
					ChunkRef: chunkRef.bytes,
				}, func(ctx context.Context, _ ids.NodeID, responseChunk *Chunk, responseErr error) {
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
						zap.Int("chunkIndex", i),
						zap.Stringer("chunkRef", chunkRef),
						zap.Stringer("nodeID", nodeID),
						zap.Int("attempt", attempt),
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

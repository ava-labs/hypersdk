// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/internal/eheap"
)

type chunkPool struct {
	pool           map[ids.ID]*ChunkCertificate
	chunkCertEHeap *eheap.ExpiryHeap[eChunk]
}

func newChunkPool() chunkPool {
	return chunkPool{
		pool:           make(map[ids.ID]*ChunkCertificate),
		chunkCertEHeap: eheap.New[eChunk](100),
	}
}

func (c *chunkPool) gatherChunkCerts() []*ChunkCertificate {
	certs := make([]*ChunkCertificate, 0, len(c.pool))
	for _, cert := range c.pool {
		certs = append(certs, cert)
	}
	return certs
}

func (c *chunkPool) add(cert *ChunkCertificate) {
	c.pool[cert.Reference.ChunkID] = cert
	c.chunkCertEHeap.Add(eChunk{
		chunkID: cert.Reference.ChunkID,
		expiry:  cert.Reference.Expiry,
	})
}

func (c *chunkPool) updateHead(block *Block) {
	for _, cert := range block.Chunks {
		delete(c.pool, cert.Reference.ChunkID)
		c.chunkCertEHeap.Remove(cert.Reference.ChunkID)
	}

	c.chunkCertEHeap.SetMin(block.Timestamp)
}

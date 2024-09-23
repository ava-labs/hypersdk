package dsmr

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

type ChunkPool interface {
	EstimateChunks() int
	StartStream(minSlot, maxSlot int64)
	Stream() (*ChunkCertificate, bool)
	FinishStreaming()
}

func GatherChunkCerts(minSlot int64, maxSlot int64, pool ChunkPool) []*ChunkCertificate {
	pool.StartStream(minSlot, maxSlot)
	defer pool.FinishStreaming()

	chunkCerts := make([]*ChunkCertificate, 0, pool.EstimateChunks())
	for cert, ok := pool.Stream(); ok; cert, ok = pool.Stream() {
		if cert.Slot < minSlot || cert.Slot > maxSlot {
			continue
		}

		chunkCerts = append(chunkCerts, cert)
	}

	return chunkCerts
}

func BuildChunkBlock(
	parentID ids.ID,
	height uint64,
	timestamp int64,
	chunkPool ChunkPool,
	backend Backend,
) (*Block, error) {
	chunks := GatherChunkCerts(timestamp, timestamp, chunkPool)
	block := &Block{
		StatelessChunkBlock: StatelessChunkBlock{
			ParentID:    parentID,
			BlockHeight: height,
			Time:        timestamp,
			Chunks:      chunks,
		},
		backend: backend,
	}
	writer := codec.NewWriter(len(chunks)*ChunkCertificateSize, consts.NetworkSizeLimit)
	if err := codec.LinearCodec.MarshalInto(block, writer.Packer); err != nil {
		return nil, err
	}

	block.bytes = writer.Bytes()
	block.id = utils.ToID(block.bytes)
	return block, nil
}

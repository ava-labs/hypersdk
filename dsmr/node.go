package dsmr

func New[T Tx](mempool Mempool[T]) *Node[T] {
	return &Node[T]{
		mempool:      mempool,
		chunkBuilder: chunkBuilder[T]{},
	}
}

type Node[T Tx] struct {
	mempool          Mempool[T]
	chunkBuilder     chunkBuilder[T]
	chunkCertBuilder chunkCertBuilder[T]
	blockBuilder     blockBuilder[T]
}

func (n Node[_]) Run(blks chan<- *Block) error {
	txs := n.mempool.GetTxsChan()

	for {
		select {
		case tx := <-txs:
			chunk, err := n.chunkBuilder.Add(tx, 0)
			if err != nil {
				return err
			}

			if chunk == nil {
				continue
			}

			chunkCert := n.chunkCertBuilder.NewCert(chunk)
			blk := n.blockBuilder.Add(chunkCert)
			if blk == nil {
				continue
			}

			blks <- blk
		}
	}
}

// consumes chunks and aggregates signtures to generate chunk certs
type chunkCertBuilder[T Tx] struct{}

// TODO implement
func (c *chunkCertBuilder[T]) NewCert(chunk *Chunk[T]) ChunkCertificate {
	return ChunkCertificate{}
}

type blockBuilder[T Tx] struct{}

func (b *blockBuilder[T]) Add(chunk ChunkCertificate) *Block {
	return &Block{}
}

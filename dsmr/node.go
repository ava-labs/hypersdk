package dsmr

import (
	"fmt"
)

func New[T Tx](client Client[T], txsPerChunk int) *Node[T] {
	return &Node[T]{
		client: client,
		chunkBuilder: chunkBuilder[T]{
			threshold: txsPerChunk,
		},
		chunks: make(chan Chunk[T], 1),
	}
}

type Node[T Tx] struct {
	//TODO chunk handler
	client           Client[T]
	chunkBuilder     chunkBuilder[T]
	chunkCertBuilder chunkCertBuilder[T]
	blockBuilder     blockBuilder[T]

	chunks chan Chunk[T]
}

func (n Node[_]) Run(blks chan<- Block) error {
	for {
		chunk := <-n.chunks

		chunkCert, err := n.chunkCertBuilder.NewCert(chunk)
		if err != nil {
			return fmt.Errorf("failed to generate chunk cert: %w", err)
		}

		blk, ok := n.blockBuilder.Add(chunkCert)
		if !ok {
			continue
		}

		blks <- blk
	}
}

// TODO why return error
// TODO handle frozen sponsor + validator assignments
// Caller is assumed to de-dup transactions?
func (n Node[T]) AddTx(tx T) error {
	chunk, err := n.chunkBuilder.Add(tx, 0)
	if err != nil {
		return err
	}

	n.chunks <- chunk
	return nil
}

func (n Node[T]) Verify(b Block) error {

}

// consumes chunks and aggregates signtures to generate chunk certs
type chunkCertBuilder[T Tx] struct{}

// TODO implement
func (c *chunkCertBuilder[T]) NewCert(chunk Chunk[T]) (ChunkCertificate, error) {
	return ChunkCertificate{}, nil
}

// TODO can this share impl w/ chunkBuilder?
type blockBuilder[T Tx] struct{}

// Add returns if a block was built
func (b *blockBuilder[T]) Add(chunk ChunkCertificate) (Block, bool) {
	return Block{}, true
}

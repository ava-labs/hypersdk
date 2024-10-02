package dsmr

func New[T Tx](chunkThreshold int) *Node[T] {
	return &Node[T]{
		chunkBuilder: chunkBuilder[T]{
			threshold: chunkThreshold,
		},
	}
}

type Node[T Tx] struct {
	chunkBuilder[T]
}

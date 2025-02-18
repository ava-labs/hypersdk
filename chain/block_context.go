package chain

var _ BlockContext = (*BlockCtx)(nil)

type BlockCtx struct {
	height    uint64
	timestamp int64
}

func (b *BlockCtx) Height() uint64 {
	return b.height
}

// Timestamp implements BlockContext.
func (b *BlockCtx) Timestamp() int64 {
	return b.timestamp
}

func NewBlockCtx(height uint64, timestamp int64) *BlockCtx {
	return &BlockCtx{
		height:    height,
		timestamp: timestamp,
	}
}

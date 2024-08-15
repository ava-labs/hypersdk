package anchor

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

type VM interface {
	HandleAnchorChunk(ctx context.Context, meta *chain.Anchor, slot int64, txs []*chain.Transaction, priorityFeeReceiverAddr codec.Address) error
	SignAnchorDigest(ctx context.Context, digest []byte) ([]byte, error)
}

package dsmr

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/utils"
)

const InitialChunkSize = 250 * 1024

type Chunk[T Tx] struct {
	Items []T   `serialize:"true"`
	Slot  int64 `serialize:"true"`

	bytes []byte
	id    ids.ID
}

func NewChunk[T Tx](items []T, slot int64) (*Chunk[T], error) {
	c := &Chunk[T]{
		Items: items,
		Slot:  slot,
	}

	packer := wrappers.Packer{Bytes: make([]byte, 0, InitialChunkSize)}
	if err := codec.LinearCodec.MarshalInto(c, &packer); err != nil {
		return nil, err
	}
	c.bytes = packer.Bytes
	c.id = utils.ToID(c.bytes)
	return c, nil
}

func (c *Chunk[T]) ID() ids.ID {
	return c.id
}

func (c *Chunk[T]) Bytes() []byte {
	return c.bytes
}

func (c *Chunk[T]) Expiry() int64 {
	return c.Slot
}

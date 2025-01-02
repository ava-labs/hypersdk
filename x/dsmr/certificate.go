// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

package dsmr

import (
	"context"
	"errors"
	"fmt"

	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"

	acodec "github.com/ava-labs/avalanchego/codec"
)

var (
	_ canoto.Message                     = (*ChunkReference)(nil)
	_ canoto.FieldMaker[*ChunkReference] = (*ChunkReference)(nil)
)

const (
	CodecVersion = 0

	MaxMessageSize = units.KiB
)

var Codec acodec.Manager

func init() {
	Codec = acodec.NewManager(MaxMessageSize)
	lc := linearcodec.NewDefault()

	err := errors.Join(
		Codec.RegisterCodec(CodecVersion, lc),
	)
	if err != nil {
		panic(err)
	}
}

type ChunkReference struct {
	ChunkID  ids.ID     `serialize:"true" canoto:"fixed bytes,1"`
	Producer ids.NodeID `serialize:"true" canoto:"fixed bytes,2"`
	Expiry   int64      `serialize:"true" canoto:"int,3"`

	canotoData canotoData_ChunkReference
}

type ChunkCertificate struct {
	ChunkReference `serialize:"true"`
	Signature      *warp.BitSetSignature `serialize:"true"`
}

func (c *ChunkCertificate) GetChunkID() ids.ID { return c.ChunkID }

func (c *ChunkCertificate) GetSlot() int64 { return c.Expiry }

func (c *ChunkCertificate) Bytes() []byte {
	bytes, err := Codec.Marshal(CodecVersion, c)
	if err != nil {
		panic(err)
	}
	return bytes
}

func (c *ChunkCertificate) Verify(
	ctx context.Context,
	networkID uint32,
	chainID ids.ID,
	pChainState validators.State,
	pChainHeight uint64,
	quorumNum uint64,
	quorumDen uint64,
) error {
	packer := wrappers.Packer{MaxSize: MaxMessageSize}
	if err := codec.LinearCodec.MarshalInto(c.ChunkReference, &packer); err != nil {
		return fmt.Errorf("failed to marshal chunk reference: %w", err)
	}

	msg, err := warp.NewUnsignedMessage(networkID, chainID, packer.Bytes)
	if err != nil {
		return fmt.Errorf("failed to initialize unsigned warp message: %w", err)
	}

	if err := c.Signature.Verify(
		ctx,
		msg,
		networkID,
		pChainState,
		pChainHeight,
		quorumNum,
		quorumDen,
	); err != nil {
		return fmt.Errorf("failed verification: %w", err)
	}

	return nil
}

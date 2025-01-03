// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//go:generate go run github.com/StephenButtolph/canoto/canoto --concurrent=false $GOFILE

package dsmr

import (
	"context"
	"fmt"

	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
)

var (
	_ canoto.Message = (*ChunkReference)(nil)
	_ canoto.Message = (*Signature)(nil)
	_ canoto.Message = (*ChunkCertificate)(nil)

	_ canoto.FieldMaker[*ChunkReference]   = (*ChunkReference)(nil)
	_ canoto.FieldMaker[*Signature]        = (*Signature)(nil)
	_ canoto.FieldMaker[*ChunkCertificate] = (*ChunkCertificate)(nil)
)

const MaxMessageSize = units.KiB

type ChunkReference struct {
	ChunkID  ids.ID     `canoto:"fixed bytes,1"`
	Producer ids.NodeID `canoto:"fixed bytes,2"`
	Expiry   int64      `canoto:"int,3"`

	canotoData canotoData_ChunkReference
}

type Signature struct {
	// Signers is a big-endian byte slice encoding which validators signed this
	// message.
	Signers   []byte                 `canoto:"bytes,1"`
	Signature [bls.SignatureLen]byte `canoto:"fixed bytes,2"`

	canotoData canotoData_Signature
}

type ChunkCertificate struct {
	ChunkReference `canoto:"value,1"`
	Signature      Signature `canoto:"value,2"`

	canotoData canotoData_ChunkCertificate
}

func (c *ChunkCertificate) GetChunkID() ids.ID { return c.ChunkID }

func (c *ChunkCertificate) GetSlot() int64 { return c.Expiry }

func (c *ChunkCertificate) Verify(
	ctx context.Context,
	networkID uint32,
	chainID ids.ID,
	pChainState validators.State,
	pChainHeight uint64,
	quorumNum uint64,
	quorumDen uint64,
) error {
	chunkReferenceBytes := c.ChunkReference.MarshalCanoto()
	msg, err := warp.NewUnsignedMessage(networkID, chainID, chunkReferenceBytes)
	if err != nil {
		return fmt.Errorf("failed to initialize unsigned warp message: %w", err)
	}

	signature := warp.BitSetSignature{
		Signers:   c.Signature.Signers,
		Signature: c.Signature.Signature,
	}
	if err := signature.Verify(
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

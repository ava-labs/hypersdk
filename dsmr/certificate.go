// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"

	acodec "github.com/ava-labs/avalanchego/codec"
)

const (
	CodecVersion = 0

	MaxMessageSize = units.KiB
)

var (
	Codec acodec.Manager

	_ ChunkCertificate[interface{}]                  = (*NoVerifyChunkCertificate)(nil)
	_ ChunkCertificate[WarpChunkVerificationContext] = (*WarpChunkCertificate)(nil)
)

func init() {
	Codec = acodec.NewManager(MaxMessageSize)
	lc := linearcodec.NewDefault()

	err := errors.Join(
		lc.RegisterType(&WarpChunkCertificate{}),
		Codec.RegisterCodec(CodecVersion, lc),
	)
	if err != nil {
		panic(err)
	}
}

type ChunkCertificate[VerificationContext any] interface {
	GetChunkID() ids.ID
	GetSlot() int64
	Bytes() []byte
	Verify(context.Context, VerificationContext) error
}

type NoVerifyChunkSignatureShare struct{}

func (NoVerifyChunkSignatureShare) Verify(_ ids.ID) error { return nil }

type NoVerifyChunkSignature struct{}

func (NoVerifyChunkSignature) Verify() error { return nil }

type NoVerifyChunkCertificate struct {
	ChunkID ids.ID `serialize:"true"`
	Slot    int64  `serialize:"true"`

	Signature NoVerifyChunkSignature `serialize:"true"`
}

func (n *NoVerifyChunkCertificate) GetChunkID() ids.ID { return n.ChunkID }

func (n *NoVerifyChunkCertificate) GetSlot() int64 { return n.Slot }

func (n *NoVerifyChunkCertificate) Bytes() []byte {
	bytes, err := Codec.Marshal(CodecVersion, n)
	if err != nil {
		panic(err)
	}
	return bytes
}

func (n *NoVerifyChunkCertificate) Verify(_ context.Context, _ interface{}) error {
	return n.Signature.Verify()
}

type WarpChunkPayload struct {
	ChunkID ids.ID `serialize:"true"`
	Slot    int64  `serialize:"true"`

	bytes []byte
}

func NewWarpChunkPayload(chunkID ids.ID, slot int64) (*WarpChunkPayload, error) {
	payload := &WarpChunkPayload{
		ChunkID: chunkID,
		Slot:    slot,
	}

	bytes, err := Codec.Marshal(CodecVersion, payload)
	if err != nil {
		return nil, err
	}
	payload.bytes = bytes
	return payload, nil
}

func ParseWarpChunkPayload(b []byte) (*WarpChunkPayload, error) {
	warpChunkPayload := &WarpChunkPayload{bytes: b}
	if _, err := Codec.Unmarshal(warpChunkPayload.bytes, warpChunkPayload); err != nil {
		return nil, err
	}
	return warpChunkPayload, nil
}

func (p *WarpChunkPayload) Bytes() []byte { return p.bytes }

type UnsignedWarpChunkCertificate struct {
	UnsignedMessage *warp.UnsignedMessage
	AddressedCall   *payload.AddressedCall
	Payload         *WarpChunkPayload
}

func NewUnsignedWarpChunkCertificate(
	networkID uint32,
	sourceChainID ids.ID,
	chunkID ids.ID,
	slot int64,
) (*UnsignedWarpChunkCertificate, error) {
	warpChunkPayload, err := NewWarpChunkPayload(chunkID, slot)
	if err != nil {
		return nil, err
	}

	addressedCall, err := payload.NewAddressedCall([]byte{}, warpChunkPayload.Bytes())
	if err != nil {
		return nil, err
	}
	unsignedMessage, err := warp.NewUnsignedMessage(
		networkID,
		sourceChainID,
		addressedCall.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	return &UnsignedWarpChunkCertificate{
		UnsignedMessage: unsignedMessage,
		AddressedCall:   addressedCall,
		Payload:         warpChunkPayload,
	}, nil
}

func ParseUnsignedWarpChunkCertificate(b []byte) (*UnsignedWarpChunkCertificate, error) {
	unsignedMessage, err := warp.ParseUnsignedMessage(b)
	if err != nil {
		return nil, err
	}

	addressedCall, err := payload.ParseAddressedCall(unsignedMessage.Payload)
	if err != nil {
		return nil, err
	}
	if len(addressedCall.SourceAddress) != 0 {
		return nil, fmt.Errorf("failed to parse system source address: %x", addressedCall.SourceAddress)
	}

	warpChunkPayload, err := ParseWarpChunkPayload(addressedCall.Payload)
	if err != nil {
		return nil, err
	}

	return &UnsignedWarpChunkCertificate{
		UnsignedMessage: unsignedMessage,
		AddressedCall:   addressedCall,
		Payload:         warpChunkPayload,
	}, nil
}

func (c *UnsignedWarpChunkCertificate) ChunkID() ids.ID {
	return c.Payload.ChunkID
}

func (c *UnsignedWarpChunkCertificate) Slot() int64 {
	return c.Payload.Slot
}

func (c *UnsignedWarpChunkCertificate) Bytes() []byte { return c.UnsignedMessage.Bytes() }

type WarpChunkCertificate struct {
	UnsignedCertificate *UnsignedWarpChunkCertificate
	Message             *warp.Message
}

func NewWarpChunkCertificate(
	unsignedCertificate *UnsignedWarpChunkCertificate,
	signature warp.Signature,
) (*WarpChunkCertificate, error) {
	msg, err := warp.NewMessage(unsignedCertificate.UnsignedMessage, signature)
	if err != nil {
		return nil, err
	}
	return &WarpChunkCertificate{
		UnsignedCertificate: unsignedCertificate,
		Message:             msg,
	}, nil
}

func ParseWarpChunkCertificate(b []byte) (*WarpChunkCertificate, error) {
	msg, err := warp.ParseMessage(b)
	if err != nil {
		return nil, err
	}

	cert, err := ParseUnsignedWarpChunkCertificate(msg.UnsignedMessage.Bytes())
	if err != nil {
		return nil, err
	}

	return &WarpChunkCertificate{
		UnsignedCertificate: cert,
		Message:             msg,
	}, nil
}

func (c *WarpChunkCertificate) GetChunkID() ids.ID { return c.UnsignedCertificate.ChunkID() }

func (c *WarpChunkCertificate) GetSlot() int64 { return c.UnsignedCertificate.Slot() }

func (c *WarpChunkCertificate) Bytes() []byte { return c.Message.Bytes() }

type WarpChunkVerificationContext struct {
	NetworkID    uint32
	PChainState  validators.State
	PChainHeight uint64
	QuorumNum    uint64
	QuorumDen    uint64
}

func (c *WarpChunkCertificate) Verify(
	ctx context.Context,
	verificationContext WarpChunkVerificationContext,
) error {
	return c.Message.Signature.Verify(
		ctx,
		c.UnsignedCertificate.UnsignedMessage,
		verificationContext.NetworkID,
		verificationContext.PChainState,
		verificationContext.PChainHeight,
		verificationContext.QuorumNum,
		verificationContext.QuorumDen,
	)
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
)

// Inspired by: https://yourbasic.org/golang/bitmask-flag-set-clear
type bits uint8

type OptionalPacker struct {
	b      bits
	offset uint8
	ip     *Packer
}

func NewOptionalWriter() *OptionalPacker {
	return &OptionalPacker{
		ip: NewWriter(consts.MaxInt),
	}
}

// used when decoding
func (p *Packer) NewOptionalReader() *OptionalPacker {
	o := &OptionalPacker{
		ip: p,
	}
	o.b = bits(o.ip.UnpackByte())
	return o
}

func (o *OptionalPacker) marker() bits {
	return 1 << o.offset
}

func (o *OptionalPacker) setBit() {
	if o.offset > consts.MaxUint8Offset {
		o.ip.addErr(ErrTooManyItems)
		return
	}
	o.b |= o.marker()
	o.offset++
}

func (o *OptionalPacker) skipBit() {
	if o.offset > consts.MaxUint8Offset {
		o.ip.addErr(ErrTooManyItems)
		return
	}
	o.offset++
}

func (o *OptionalPacker) checkBit() bool {
	result := o.b&o.marker() != 0
	o.offset++
	return result
}

func (o *OptionalPacker) PackID(id ids.ID) {
	if id == ids.Empty {
		o.skipBit()
		return
	}
	o.ip.PackID(id)
	o.setBit()
}

func (o *OptionalPacker) UnpackID(dest *ids.ID) {
	if o.checkBit() {
		o.ip.UnpackID(true, dest)
	}
}

func (o *OptionalPacker) PackPublicKey(pk crypto.PublicKey) {
	if pk == crypto.EmptyPublicKey {
		o.skipBit()
		return
	}
	o.ip.PackPublicKey(pk)
	o.setBit()
}

func (o *OptionalPacker) UnpackPublicKey(dest *crypto.PublicKey) {
	if o.checkBit() {
		o.ip.UnpackPublicKey(dest)
	}
}

func (o *OptionalPacker) PackUint64(l uint64) {
	if l == 0 {
		o.skipBit()
		return
	}
	o.ip.PackUint64(l)
	o.setBit()
}

func (o *OptionalPacker) UnpackUint64() uint64 {
	if o.checkBit() {
		return o.ip.UnpackUint64(true)
	}
	return 0
}

func (p *Packer) PackOptional(o *OptionalPacker) {
	p.PackByte(uint8(o.b))
	p.PackFixedBytes(o.ip.Bytes())
}

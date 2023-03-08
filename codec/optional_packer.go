// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
)

// Represents a bitmask.
// Inspired by: https://yourbasic.org/golang/bitmask-flag-set-clear
type bits uint8

type OptionalPacker struct {
	b      bits
	offset uint8
	ip     *Packer
}

// NewOptionalWriter returns an instance of OptionalPacker that includes
// a new Packer instance with MaxSize set to the maximum size.
func NewOptionalWriter() *OptionalPacker {
	return &OptionalPacker{
		ip: NewWriter(consts.MaxInt),
	}
}

// NewOptionalReader returns an instance of OptionalPacker that includes
// a packer instance set to [p]. It sets the packers bits b to the value of the packers
// UnpackByte.
//
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

// setBit sets the OptionalPacker's bitmask at o.offset and increments
// the offset. If offset exceeds the maximum offset, setBit returns without
// updating the bitmask and adds an error to the Packer.
func (o *OptionalPacker) setBit() {
	if o.offset > consts.MaxUint8Offset {
		o.ip.addErr(ErrTooManyItems)
		return
	}
	o.b |= o.marker()
	o.offset++
}

// setBit increments the bitmasks offset. If offset already exceeds the maximum
// offset, setBit returns and adds an error to the Packer.
func (o *OptionalPacker) skipBit() {
	if o.offset > consts.MaxUint8Offset {
		o.ip.addErr(ErrTooManyItems)
		return
	}
	o.offset++
}

// checkBit returns whether the OptionalPacker's bitmask is true at the current
// offset. Increments the offset.
func (o *OptionalPacker) checkBit() bool {
	result := o.b&o.marker() != 0
	o.offset++
	return result
}

// PackID packs [id] into o if it is not empty.
// Updates the bitmask and offset accordingly.
func (o *OptionalPacker) PackID(id ids.ID) {
	if id == ids.Empty {
		o.skipBit()
		return
	}
	o.ip.PackID(id)
	o.setBit()
}

// UnpackID unpacks an id into [dest] if the bitmask is set at the current offset.
// Increments offset regardless.
func (o *OptionalPacker) UnpackID(dest *ids.ID) {
	if o.checkBit() {
		o.ip.UnpackID(true, dest)
	}
}

// PackPublicKey packs [pk] into o if [pk] is not an empty PublicKey.
// Updates the bitmask and offset accordingly.
func (o *OptionalPacker) PackPublicKey(pk crypto.PublicKey) {
	if pk == crypto.EmptyPublicKey {
		o.skipBit()
		return
	}
	o.ip.PackPublicKey(pk)
	o.setBit()
}

// UnpackPublicKey unpacks a PublicKey into [dest] if the bitmask is set at
// the current offset. Increments offset regardless.
func (o *OptionalPacker) UnpackPublicKey(dest *crypto.PublicKey) {
	if o.checkBit() {
		o.ip.UnpackPublicKey(true, dest)
	}
}

// PackUint64 packs [l] into o if [l] is not an 0.
// Updates the bitmask and offset accordingly.
func (o *OptionalPacker) PackUint64(l uint64) {
	if l == 0 {
		o.skipBit()
		return
	}
	o.ip.PackUint64(l)
	o.setBit()
}

// UnpackUint64 unpacks a Uint64 from o and returns the value.
// Increments offset regardless.
func (o *OptionalPacker) UnpackUint64() uint64 {
	if o.checkBit() {
		return o.ip.UnpackUint64(true)
	}
	return 0
}

// PackOptional packs an OptionalPacker in a Packer. First packs the bitmask [0.b]
// followed by the bytes in the OptionalPacker.
func (p *Packer) PackOptional(o *OptionalPacker) {
	p.PackByte(uint8(o.b))
	p.PackFixedBytes(o.ip.Bytes())
}

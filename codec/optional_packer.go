// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"math"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
)

// MaxItems stores the maximum items an optional packer can hold.
// Since a bitset can range from [0, INT_MAX] the max number of
// flags in a bitset is log2(INT_MAX).
var MaxItems = uint8(math.Log2(float64(consts.MaxInt)))

// OptionalPacker defines a struct that includes a Packer [ip], a bitmask
// [b] and an offset [offset]. [b] indicates which fields in the OptionalPacker
// are present and which are not.
type OptionalPacker struct {
	b        set.Bits
	MaxItems uint8
	offset   uint8
	ip       *Packer
}

// NewOptionalWriter returns an instance of OptionalPacker that includes
// a new Packer instance with MaxSize set to the maximum size. The maximum items
// OptionalPacker can hold is set to [size]
func NewOptionalWriter(size uint8) *OptionalPacker {
	return &OptionalPacker{
		ip:       NewWriter(consts.MaxInt),
		b:        set.NewBits(),
		MaxItems: size,
	}
}

// NewOptionalReader returns an instance of OptionalPacker that includes
// a packer instance set to [p]. It sets the packers bits b to the value of the packers
// UnpackByte and the MaxItems to [size]
//
// used when decoding
func (p *Packer) NewOptionalReader(size uint8) *OptionalPacker {
	o := &OptionalPacker{
		ip:       p,
		MaxItems: size,
	}
	o.b = set.BitsFromBytes([]byte{o.ip.UnpackByte()})
	return o
}

// func (o *OptionalPacker) marker() bits {
// 	return 1 << o.offset
// }

// setBit sets the OptionalPacker's bitmask at o.offset and increments
// the offset. If offset exceeds the maximum offset, setBit returns without
// updating the bitmask and adds an error to the Packer.
func (o *OptionalPacker) setBit() {
	if o.offset >= o.MaxItems {
		o.ip.addErr(ErrTooManyItems)
		return
	}
	o.b.Add(int(o.offset))
	o.offset++
}

// setBit increments the bitmasks offset. If offset already exceeds the maximum
// offset, setBit returns and adds an error to the Packer.
func (o *OptionalPacker) skipBit() {
	if o.offset >= o.MaxItems {
		o.ip.addErr(ErrTooManyItems)
		return
	}
	o.offset++
}

// checkBit returns whether the OptionalPacker's bitmask is true at the current
// offset. Increments the offset.
func (o *OptionalPacker) checkBit() bool {
	result := o.b.Contains(int(o.offset))
	o.offset++
	return result
}

// PackID packs [id] into OptionalPacker if it is not empty.
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
	} else {
		*dest = ids.Empty
	}
}

// PackPublicKey packs [pk] into OptionalPacker if [pk] is not an empty PublicKey.
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
	} else {
		*dest = crypto.EmptyPublicKey
	}
}

// PackUint64 packs [l] into OptionalPacker if [l] is not an 0.
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

// PackOptional packs an OptionalPacker in a Packer. First packs the bitmask [o.b]
// followed by the bytes in the OptionalPacker.
func (p *Packer) PackOptional(o *OptionalPacker) {
	if o.b.Len() == 0 {
		p.PackByte(0)
	} else {
		p.PackFixedBytes(o.b.Bytes())
	}
	p.PackFixedBytes(o.ip.Bytes())
}

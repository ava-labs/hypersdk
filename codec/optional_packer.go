// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/consts"
)

// OptionalPacker defines a struct that includes a Packer [ip], a bitset
// [b] and an offset [offset]. [b] indicates which fields in the OptionalPacker
// are present and which are not.
type OptionalPacker struct {
	b      set.Bits64
	offset uint8
	ip     *Packer
}

// NewOptionalWriter returns an instance of OptionalPacker that includes
// a new Packer instance with MaxSize set to the maximum size. The maximum items
// OptionalPacker can hold is set to [size]. If [size] > MaxItems sets
// OptionalPackers MaxItems to MaxItems
func NewOptionalWriter(initial int) *OptionalPacker {
	return &OptionalPacker{
		ip: NewWriter(initial, consts.MaxInt),
	}
}

// NewOptionalReader returns an instance of OptionalPacker that includes
// a packer instance set to [p]. It sets the packers bits b to the value of the packers
// UnpackByte and the MaxItems to [size]. If [size] > MaxItems sets
// OptionalPackers MaxItems to MaxItems
//
// used when decoding
func (p *Packer) NewOptionalReader() *OptionalPacker {
	o := &OptionalPacker{
		ip: p,
	}
	o.b = set.Bits64(o.ip.UnpackUint64(false))
	return o
}

// setBit sets the OptionalPacker's bitset at o.offset and increments
// the offset. If offset exceeds the maximum offset, setBit returns without
// updating the bitset and adds an error to the Packer.
func (o *OptionalPacker) setBit() {
	if o.offset > consts.MaxUint64Offset {
		o.ip.addErr(ErrTooManyItems)
		return
	}
	o.b.Add(uint(o.offset))
	o.offset++
}

// skipBit increments the offset. If offset already exceeds the maximum
// offset, setBit returns and adds an error to the Packer.
func (o *OptionalPacker) skipBit() {
	if o.offset > consts.MaxUint64Offset {
		o.ip.addErr(ErrTooManyItems)
		return
	}
	o.offset++
}

// checkBit returns whether the OptionalPacker's bitset is true at the current
// offset. Increments the offset.
func (o *OptionalPacker) checkBit() bool {
	result := o.b.Contains(uint(o.offset))
	o.offset++
	return result
}

// PackID packs [id] into OptionalPacker if it is not empty.
// Updates the bitset and offset accordingly.
func (o *OptionalPacker) PackID(id ids.ID) {
	if id == ids.Empty {
		o.skipBit()
		return
	}
	o.ip.PackID(id)
	o.setBit()
}

// UnpackID unpacks an id into [dest] if the bitset is set at the current offset.
// Increments offset regardless.
func (o *OptionalPacker) UnpackID(dest *ids.ID) {
	if o.checkBit() {
		o.ip.UnpackID(true, dest)
	} else {
		*dest = ids.Empty
	}
}

// PackUint64 packs [l] into OptionalPacker if [l] is not an 0.
// Updates the bitset and offset accordingly.
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

// PackInt64 packs [l] into OptionalPacker if [l] is not an 0.
// Updates the bitset and offset accordingly.
func (o *OptionalPacker) PackInt64(l int64) {
	if l == 0 {
		o.skipBit()
		return
	}
	o.ip.PackInt64(l)
	o.setBit()
}

// UnpackInt64 unpacks a Int64 from o and returns the value.
// Increments offset regardless.
func (o *OptionalPacker) UnpackInt64() int64 {
	if o.checkBit() {
		return o.ip.UnpackInt64(true)
	}
	return 0
}

// PackAddress packs [addr] into OptionalPacker if [addr] is not empty.
// Updates the bitset and offset accordingly.
func (o *OptionalPacker) PackAddress(addr Address) {
	if addr == EmptyAddress {
		o.skipBit()
		return
	}
	o.ip.PackAddress(addr)
	o.setBit()
}

// UnpackAddress unpacks Address into [dest] if the bitset is set at
// the current offset. Increments offset regardless.
func (o *OptionalPacker) UnpackAddress(dest *Address) {
	if o.checkBit() {
		o.ip.UnpackAddress(dest)
	} else {
		*dest = EmptyAddress
	}
}

// PackOptional packs an OptionalPacker in a Packer. First packs the bitset [o.b]
// followed by the bytes in the OptionalPacker.
func (p *Packer) PackOptional(o *OptionalPacker) {
	p.PackUint64(uint64(o.b))
	p.PackFixedBytes(o.ip.Bytes())
}

// Done is called when done reading items from an OptionalPacker. It asserts
// that no bits are populated above the largest read offset.
func (o *OptionalPacker) Done() {
	if o.offset == consts.MaxUint64Offset+1 {
		// When checking the MaxUint64Offset item, we increment the offset once
		// more.
		return
	}
	var maxSet set.Bits64
	maxSet.Add(uint(o.offset))
	if o.b < maxSet {
		return
	}
	o.ip.addErr(ErrInvalidBitset)
}

// Err returns any error associated with the inner Packer.
func (o *OptionalPacker) Err() error {
	return o.ip.Err()
}

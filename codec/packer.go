// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"fmt"

	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/internal/window"
)

var LinearCodec = linearcodec.NewDefault()

// Packer is a wrapper struct for the Packer struct
// from avalanchego/utils/wrappers/packing.go. A bool [required] parameter is
// added to many unpacking methods, which signals the packer to add an error
// if the expected method does not unpack properly.
type Packer struct {
	*wrappers.Packer
}

// NewReader returns a Packer instance with the current byte array set to [byte]
// and it's MaxSize set to [limit].
func NewReader(src []byte, limit int) *Packer {
	return &Packer{
		Packer: &wrappers.Packer{Bytes: src, MaxSize: limit},
	}
}

// NewWriter returns a Packer instance with an initial size of [initial] and a
// MaxSize set to [limit].
func NewWriter(initial, limit int) *Packer {
	return &Packer{
		Packer: &wrappers.Packer{Bytes: make([]byte, 0, initial), MaxSize: limit},
	}
}

func (p *Packer) PackID(src ids.ID) {
	p.Packer.PackFixedBytes(src[:])
}

// UnpackID unpacks an avalanchego ID into [dest]. If [required] is true,
// and the unpacked bytes are empty, Packer will add an ErrFieldNotPopulated error.
func (p *Packer) UnpackID(required bool, dest *ids.ID) {
	copy((*dest)[:], p.Packer.UnpackFixedBytes(ids.IDLen))
	if required && *dest == ids.Empty {
		p.addErr(fmt.Errorf("%w: ID field is not populated", ErrFieldNotPopulated))
	}
}

func (p *Packer) PackAddress(a Address) {
	p.Packer.PackFixedBytes(a[:])
}

func (p *Packer) UnpackAddress(dest *Address) {
	copy((*dest)[:], p.Packer.UnpackFixedBytes(AddressLen))
	if *dest == EmptyAddress {
		p.addErr(fmt.Errorf("%w: Address field is not populated", ErrFieldNotPopulated))
	}
}

func (p *Packer) UnpackFixedBytes(size int, dest *[]byte) {
	copy((*dest), p.Packer.UnpackFixedBytes(size))
}

// UnpackBytes unpacks [limit] bytes into [dest]. Otherwise
// if [limit] >= 0, UnpackBytes unpacks a byte slice array into [dest]. If
// [required] is set to true and the amount of bytes written to [dest] is 0,
// UnpackBytes adds an err ErrFieldNotPopulated to the Packer.
func (p *Packer) UnpackBytes(limit int, required bool, dest *[]byte) {
	if limit >= 0 {
		*dest = p.Packer.UnpackLimitedBytes(uint32(limit))
	} else {
		*dest = p.Packer.UnpackBytes()
	}
	if required && len(*dest) == 0 {
		p.addErr(fmt.Errorf("%w: Bytes field is not populated", ErrFieldNotPopulated))
	}
}

func (p *Packer) PackUint64(v uint64) {
	p.Packer.PackLong(v)
}

func (p *Packer) UnpackUint64(required bool) uint64 {
	v := p.Packer.UnpackLong()
	if required && v == 0 {
		p.addErr(fmt.Errorf("%w: Uint64 field is not populated", ErrFieldNotPopulated))
	}
	return v
}

func (p *Packer) PackInt64(v int64) {
	p.Packer.PackLong(uint64(v))
}

func (p *Packer) UnpackInt64(required bool) int64 {
	v := p.Packer.UnpackLong()
	if required && v == 0 {
		p.addErr(fmt.Errorf("%w: Int64 field is not populated", ErrFieldNotPopulated))
	}
	return int64(v)
}

func (p *Packer) UnpackInt(required bool) uint32 {
	v := p.Packer.UnpackInt()
	if required && v == 0 {
		p.addErr(fmt.Errorf("%w: Int field is not populated", ErrFieldNotPopulated))
	}
	return v
}

func (p *Packer) PackWindow(w window.Window) {
	p.Packer.PackFixedBytes(w[:])
}

func (p *Packer) UnpackWindow(w *window.Window) {
	copy((*w)[:], p.Packer.UnpackFixedBytes(window.WindowSliceSize))
}

func (p *Packer) PackString(s string) {
	p.Packer.PackStr(s)
}

func (p *Packer) UnpackString(required bool) string {
	str := p.Packer.UnpackStr()
	if required && len(str) == 0 {
		p.addErr(fmt.Errorf("%w: String field is not populated", ErrFieldNotPopulated))
	}
	return str
}

// Empty is called after parsing a byte array to ensure there is nothing left
// to parse.
func (p *Packer) Empty() bool {
	return p.Packer.Offset == len(p.Packer.Bytes)
}

func (p *Packer) Err() error {
	return p.Packer.Err
}

func (p *Packer) Bytes() []byte {
	return p.Packer.Bytes
}

func (p *Packer) addErr(err error) {
	p.Packer.Errs.Add(err)
}

func (p *Packer) Offset() int {
	return p.Packer.Offset
}

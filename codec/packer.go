// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/crypto"
	"github.com/AnomalyFi/hypersdk/window"
)

// Packer is a wrapper struct for the Packer struct
// from avalanchego/utils/wrappers/packing.go. It adds methods to
// pack/unpack ids, PublicKeys and Signatures. A bool [required] parameter is
// added to many unpacking methods, which signals the packer to add an error
// if the expected method does not unpack properly.
type Packer struct {
	p *wrappers.Packer
}

// NewReader returns a Packer instance with the current byte array set to [byte]
// and it's MaxSize set to [limit].
func NewReader(src []byte, limit int) *Packer {
	return &Packer{
		p: &wrappers.Packer{Bytes: src, MaxSize: limit},
	}
}

// NewWriter returns a Packer instance with its MaxSize set to [limit].
func NewWriter(limit int) *Packer {
	return &Packer{
		p: &wrappers.Packer{MaxSize: limit},
	}
}

func (p *Packer) PackBool(src bool) {
	p.p.PackBool(src)
}

func (p *Packer) UnpackBool() bool {
	return p.p.UnpackBool()
}

func (p *Packer) PackID(src ids.ID) {
	p.p.PackFixedBytes(src[:])
}

// UnpackID unpacks an avalanchego ID into [dest]. If [required] is true,
// and the unpacked bytes are empty, Packer will add an ErrFieldNotPopulated error.
func (p *Packer) UnpackID(required bool, dest *ids.ID) {
	copy((*dest)[:], p.p.UnpackFixedBytes(consts.IDLen))
	if required && *dest == ids.Empty {
		p.addErr(fmt.Errorf("%w: ID field is not populated", ErrFieldNotPopulated))
	}
}

func (p *Packer) PackByte(b byte) {
	p.p.PackByte(b)
}

func (p *Packer) UnpackByte() byte {
	return p.p.UnpackByte()
}

func (p *Packer) PackFixedBytes(b []byte) {
	p.p.PackFixedBytes(b)
}

func (p *Packer) PackBytes(b []byte) {
	p.p.PackBytes(b)
}

// UnpackBytes unpacks [limit] bytes into [dest]. Otherwise
// if [limit] >= 0, UnpackBytes unpacks a byte slice array into [dest]. If
// [required] is set to true and the amount of bytes written to [dest] is 0,
// UnpackBytes adds an err ErrFieldNotPopulated to the Packer.
func (p *Packer) UnpackBytes(limit int, required bool, dest *[]byte) {
	if limit >= 0 {
		*dest = p.p.UnpackLimitedBytes(uint32(limit))
	} else {
		*dest = p.p.UnpackBytes()
	}
	if required && len(*dest) == 0 {
		p.addErr(fmt.Errorf("%w: Bytes field is not populated", ErrFieldNotPopulated))
	}
}

func (p *Packer) UnpackFixedBytes(size int, dest *[]byte) {
	copy((*dest), p.p.UnpackFixedBytes(size))
}

func (p *Packer) PackUint64(v uint64) {
	p.p.PackLong(v)
}

func (p *Packer) UnpackUint64(required bool) uint64 {
	v := p.p.UnpackLong()
	if required && v == 0 {
		p.addErr(fmt.Errorf("%w: Uint64 field is not populated", ErrFieldNotPopulated))
	}
	return v
}

func (p *Packer) PackInt64(v int64) {
	p.p.PackLong(uint64(v))
}

func (p *Packer) UnpackInt64(required bool) int64 {
	v := p.p.UnpackLong()
	if required && v == 0 {
		p.addErr(fmt.Errorf("%w: Int64 field is not populated", ErrFieldNotPopulated))
	}
	return int64(v)
}

func (p *Packer) PackPublicKey(src crypto.PublicKey) {
	p.p.PackFixedBytes(src[:])
}

// UnpackPublicKey crypto.PublicKey into [dest].
func (p *Packer) UnpackPublicKey(required bool, dest *crypto.PublicKey) {
	copy((*dest)[:], p.p.UnpackFixedBytes(crypto.PublicKeyLen))
	if required && *dest == crypto.EmptyPublicKey {
		p.addErr(fmt.Errorf("%w: PublicKey field is not populated", ErrFieldNotPopulated))
	}
}

func (p *Packer) PackSignature(src crypto.Signature) {
	p.p.PackFixedBytes(src[:])
}

// UnpackPublicKey crypto.Signature into [dest].
// TODO: should add required param?
func (p *Packer) UnpackSignature(dest *crypto.Signature) {
	copy((*dest)[:], p.p.UnpackFixedBytes(crypto.SignatureLen))
	if *dest == crypto.EmptySignature {
		p.addErr(fmt.Errorf("%w: Signature field is not populated", ErrFieldNotPopulated))
	}
}

func (p *Packer) PackInt(v int) {
	p.p.PackInt(uint32(v))
}

func (p *Packer) UnpackInt(required bool) int {
	v := p.p.UnpackInt()
	if required && v == 0 {
		p.addErr(fmt.Errorf("%w: Int field is not populated", ErrFieldNotPopulated))
	}
	return int(v)
}

func (p *Packer) PackWindow(w window.Window) {
	p.p.PackFixedBytes(w[:])
}

func (p *Packer) UnpackWindow(w *window.Window) {
	copy((*w)[:], p.p.UnpackFixedBytes(window.WindowSliceSize))
}

func (p *Packer) PackString(s string) {
	p.p.PackStr(s)
}

func (p *Packer) UnpackString(required bool) string {
	str := p.p.UnpackStr()
	if required && len(str) == 0 {
		p.addErr(fmt.Errorf("%w: String field is not populated", ErrFieldNotPopulated))
	}
	return str
}

// Empty is called after parsing a byte array to ensure there is nothing left
// to parse.
func (p *Packer) Empty() bool {
	return p.p.Offset == len(p.p.Bytes)
}

func (p *Packer) Err() error {
	return p.p.Err
}

func (p *Packer) Bytes() []byte {
	return p.p.Bytes
}

func (p *Packer) addErr(err error) {
	p.p.Errs.Add(err)
}

func (p *Packer) Offset() int {
	return p.p.Offset
}

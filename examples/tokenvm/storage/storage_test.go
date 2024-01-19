// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	_ "embed"
	"encoding/binary"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

func serializeOriginal(
	asset ids.ID,
	symbol []byte,
	decimals uint8,
	metadata []byte,
	supply uint64,
	owner codec.Address,
	warp bool,
) []byte {
	symbolLen := len(symbol)
	metadataLen := len(metadata)
	v := make([]byte, consts.Uint32Len+symbolLen+consts.Uint8Len+consts.Uint32Len+metadataLen+consts.Uint64Len+codec.AddressLen+1)
	binary.BigEndian.PutUint32(v, uint32(symbolLen))
	copy(v[consts.Uint32Len:], symbol)
	v[consts.Uint32Len+symbolLen] = decimals
	binary.BigEndian.PutUint32(v[consts.Uint32Len+symbolLen+consts.Uint8Len:], uint32(metadataLen))
	copy(v[consts.Uint16Len+symbolLen+consts.Uint8Len+consts.Uint16Len:], metadata)
	binary.BigEndian.PutUint64(v[consts.Uint32Len+symbolLen+consts.Uint8Len+consts.Uint32Len+metadataLen:], supply)
	copy(v[consts.Uint32Len+symbolLen+consts.Uint8Len+consts.Uint32Len+metadataLen+consts.Uint64Len:], owner[:])
	b := byte(0x0)
	if warp {
		b = 0x1
	}
	v[consts.Uint32Len+symbolLen+consts.Uint8Len+consts.Uint32Len+metadataLen+consts.Uint64Len+codec.AddressLen] = b

	return v
}

func serializeWithPacker(
	asset ids.ID,
	symbol []byte,
	decimals uint8,
	metadata []byte,
	supply uint64,
	owner codec.Address,
	warp bool,
) ([]byte, error) {
	symbolLen := len(symbol)
	metadataLen := len(metadata)
	p := codec.NewWriter(consts.Uint32Len+symbolLen+consts.Uint8Len+consts.Uint32Len+metadataLen+consts.Uint64Len+codec.AddressLen+consts.ByteLen, consts.MaxInt)

	p.PackBytes(symbol)
	p.PackByte(decimals)
	p.PackBytes(metadata)
	p.PackUint64(supply)
	p.PackAddress(owner)
	p.PackBool(warp)

	if err := p.Err(); err != nil {
		return nil, err
	}

	return p.Bytes(), nil
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkSerializeOriginal$ github.com/ava-labs/hypersdk/examples/tokenvm/storage -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkSerializeOriginal(b *testing.B) {
	asset := ids.GenerateTestID()
	symbol := []byte("AVAX")
	decimals := uint8(16)
	metadata := []byte("Avalanche")
	supply := uint64(72000000000000000)
	owner := codec.Address{}
	warp := false

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		serializeOriginal(asset, symbol, decimals, metadata, supply, owner, warp)
	}
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkSerializeWithPacker$ github.com/ava-labs/hypersdk/examples/tokenvm/storage -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkSerializeWithPacker(b *testing.B) {
	asset := ids.GenerateTestID()
	symbol := []byte("AVAX")
	decimals := uint8(16)
	metadata := []byte("Avalanche")
	supply := uint64(72000000000000000)
	owner := codec.Address{}
	warp := false

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		serializeWithPacker(asset, symbol, decimals, metadata, supply, owner, warp)
	}
}

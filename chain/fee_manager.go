package chain

import (
	"encoding/binary"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/window"
)

const (
	bandwidth           int = 0
	compute                 = 1
	storageRead             = 2
	storageCreate           = 3
	storageModification     = 4

	dimensionSize = consts.Uint64Len + window.WindowSliceSize + consts.Uint64Len
)

type FeeManager struct {
	raw []byte
}

func (f *FeeManager) UnitPrice(dimension int) uint64 {
	start := dimensionSize * dimension
	return binary.BigEndian.Uint64(f.raw[start : start+consts.Uint64Len])
}

func (f *FeeManager) Window(dimension int) window.Window {
	start := dimensionSize*dimension + consts.Uint64Len
	return window.Window(f.raw[start : start+window.WindowSliceSize])
}

func (f *FeeManager) LastConsumed(dimension int) uint64 {
	start := dimensionSize*dimension + consts.Uint64Len + window.WindowSliceSize
	return binary.BigEndian.Uint64(f.raw[start : start+consts.Uint64Len])
}

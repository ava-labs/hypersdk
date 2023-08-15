package chain

import (
	"encoding/binary"

	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/window"
)

const (
	bandwidth           int = 0
	compute                 = 1
	storageRead             = 2
	storageCreate           = 3
	storageModification     = 4

	dimensions = 5

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

func computeNextPriceWindow(
	previous window.Window,
	previousConsumed uint64,
	previousPrice uint64,
	target uint64, /* per window */
	changeDenom uint64,
	minPrice uint64,
	since int, /* seconds */
) (uint64, window.Window, error) {
	newRollupWindow, err := window.Roll(previous, since)
	if err != nil {
		return 0, window.Window{}, err
	}
	if since < window.WindowSize {
		// add in the units used by the parent block in the correct place
		// If the parent consumed units within the rollup window, add the consumed
		// units in.
		slot := window.WindowSize - 1 - since
		start := slot * consts.Uint64Len
		window.Update(&newRollupWindow, start, previousConsumed)
	}
	total := window.Sum(newRollupWindow)

	nextPrice := previousPrice
	if total > target {
		// If the parent block used more units than its target, the baseFee should increase.
		delta := total - target
		x := previousPrice * delta
		y := x / target
		baseDelta := y / changeDenom
		if baseDelta < 1 {
			baseDelta = 1
		}
		n, over := math.Add64(nextPrice, baseDelta)
		if over != nil {
			nextPrice = consts.MaxUint64
		} else {
			nextPrice = n
		}
	} else if total < target {
		// Otherwise if the parent block used less units than its target, the baseFee should decrease.
		delta := target - total
		x := previousPrice * delta
		y := x / target
		baseDelta := y / changeDenom
		if baseDelta < 1 {
			baseDelta = 1
		}

		// If [roll] is greater than [rollupWindow], apply the state transition to the base fee to account
		// for the interval during which no blocks were produced.
		// We use roll/rollupWindow, so that the transition is applied for every [rollupWindow] seconds
		// that has elapsed between the parent and this block.
		if since > window.WindowSize {
			// Note: roll/rollupWindow must be greater than 1 since we've checked that roll > rollupWindow
			baseDelta *= uint64(since / window.WindowSize)
		}
		n, under := math.Sub(nextPrice, baseDelta)
		if under != nil {
			nextPrice = 0
		} else {
			nextPrice = n
		}
	}
	if nextPrice < minPrice {
		nextPrice = minPrice
	}
	return nextPrice, newRollupWindow, nil
}

func (f *FeeManager) ComputeNext(lastTime int64, currTime int64, r Rules) (*FeeManager, error) {
	targetUnits := r.GetWindowTargetUnits()
	unitPriceChangeDenom := r.GetUnitPriceChangeDenominator()
	minUnitPrice := r.GetMinUnitPrice()

	since := int((currTime - lastTime) / consts.MillisecondsPerSecond)
	newRaw := []byte{}
	for i := 0; i < dimensions; i++ {
		nextUnitPrice, nextUnitWindow, err := computeNextPriceWindow(
			f.Window(i),
			f.LastConsumed(i),
			f.UnitPrice(i),
			targetUnits[i],
			unitPriceChangeDenom[i],
			minUnitPrice[i],
			since,
		)
		if err != nil {
			return nil, err
		}

		// TODO: add items to raw
	}
}

func (f *FeeManager) SetLastConsumed(dimension int, consumed uint64) {
	start := dimensionSize*dimension + consts.Uint64Len + window.WindowSliceSize
	binary.BigEndian.PutUint64(f.raw[start:start+consts.Uint64Len], consumed)
}

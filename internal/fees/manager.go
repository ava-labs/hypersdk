// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"encoding/binary"
	"sync"

	"github.com/ava-labs/avalanchego/utils/math"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/window"
)

const dimensionStateLen = consts.Uint64Len + window.WindowSliceSize + consts.Uint64Len

// Manager is safe for concurrent use
type Manager struct {
	l sync.RWMutex

	// Layout: [timestamp(s)][dimension[0].price][dimension[0].window][dimension[0].lastConsumed]...
	//
	// Note, we must store the timestamp because computing the time difference exclusively off
	// of block parent timestamps may never result in a window roll (difference may always be less
	// than 1 second). This bug would result in the unit price never changing (or even going up if the
	// last consumed is larger than the target).
	raw []byte
}

func NewManager(raw []byte) *Manager {
	if len(raw) == 0 {
		raw = make([]byte, consts.Int64Len+fees.FeeDimensions*dimensionStateLen)
	}
	return &Manager{raw: raw}
}

func (f *Manager) UnitPrice(d fees.Dimension) uint64 {
	f.l.RLock()
	defer f.l.RUnlock()

	return f.unitPrice(d)
}

func (f *Manager) unitPrice(d fees.Dimension) uint64 {
	start := consts.Int64Len + dimensionStateLen*d
	return binary.BigEndian.Uint64(f.raw[start : start+consts.Uint64Len])
}

func (f *Manager) Window(d fees.Dimension) window.Window {
	f.l.RLock()
	defer f.l.RUnlock()

	return f.window(d)
}

func (f *Manager) window(d fees.Dimension) window.Window {
	start := consts.Int64Len + dimensionStateLen*d + consts.Uint64Len
	return window.Window(f.raw[start : start+window.WindowSliceSize])
}

func (f *Manager) LastConsumed(d fees.Dimension) uint64 {
	f.l.RLock()
	defer f.l.RUnlock()

	return f.lastConsumed(d)
}

func (f *Manager) lastConsumed(d fees.Dimension) uint64 {
	start := consts.Int64Len + dimensionStateLen*d + consts.Uint64Len + window.WindowSliceSize
	return binary.BigEndian.Uint64(f.raw[start : start+consts.Uint64Len])
}

func (f *Manager) ComputeNext(currTime int64, r Rules) *Manager {
	f.l.RLock()
	defer f.l.RUnlock()

	targetUnits := r.GetWindowTargetUnits()
	unitPriceChangeDenom := r.GetUnitPriceChangeDenominator()
	minUnitPrice := r.GetMinUnitPrice()
	lastTimeSeconds := int64(binary.BigEndian.Uint64(f.raw[0:consts.Int64Len]))
	currTimeSeconds := currTime / consts.MillisecondsPerSecond
	since := uint64(currTimeSeconds - lastTimeSeconds)
	bytes := make([]byte, consts.Int64Len+dimensionStateLen*fees.FeeDimensions)
	binary.BigEndian.PutUint64(bytes[0:consts.Int64Len], uint64(currTimeSeconds))
	for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
		nextUnitPrice, nextUnitWindow := computeNextPriceWindow(
			f.Window(i),
			f.LastConsumed(i),
			f.UnitPrice(i),
			targetUnits[i],
			unitPriceChangeDenom[i],
			minUnitPrice[i],
			since,
		)
		start := consts.Int64Len + dimensionStateLen*i
		binary.BigEndian.PutUint64(bytes[start:start+consts.Uint64Len], nextUnitPrice)
		copy(bytes[start+consts.Uint64Len:start+consts.Uint64Len+window.WindowSliceSize], nextUnitWindow[:])
		// Usage must be set after block is processed (we leave as 0 for now)
	}
	return &Manager{raw: bytes}
}

func (f *Manager) SetUnitPrice(d fees.Dimension, price uint64) {
	f.l.Lock()
	defer f.l.Unlock()

	f.setUnitPrice(d, price)
}

func (f *Manager) setUnitPrice(d fees.Dimension, price uint64) {
	start := consts.Int64Len + dimensionStateLen*d
	binary.BigEndian.PutUint64(f.raw[start:start+consts.Uint64Len], price)
}

func (f *Manager) SetLastConsumed(d fees.Dimension, consumed uint64) {
	f.l.Lock()
	defer f.l.Unlock()

	f.setLastConsumed(d, consumed)
}

func (f *Manager) setLastConsumed(d fees.Dimension, consumed uint64) {
	start := consts.Int64Len + dimensionStateLen*d + consts.Uint64Len + window.WindowSliceSize
	binary.BigEndian.PutUint64(f.raw[start:start+consts.Uint64Len], consumed)
}

func (f *Manager) Consume(d fees.Dimensions, l fees.Dimensions) (bool, fees.Dimension) {
	f.l.Lock()
	defer f.l.Unlock()

	// Ensure we can consume (don't want partial update of values)
	for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
		consumed, err := math.Add(f.lastConsumed(i), d[i])
		if err != nil {
			return false, i
		}
		if consumed > l[i] {
			return false, i
		}
	}

	// Commit to consumption
	for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
		consumed, err := math.Add(f.lastConsumed(i), d[i])
		if err != nil {
			return false, i
		}
		f.setLastConsumed(i, consumed)
	}
	return true, 0
}

func (f *Manager) Bytes() []byte {
	f.l.RLock()
	defer f.l.RUnlock()

	return f.raw
}

func (f *Manager) Fee(d fees.Dimensions) (uint64, error) {
	f.l.RLock()
	defer f.l.RUnlock()

	fee := uint64(0)
	for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
		contribution, err := math.Mul(f.unitPrice(i), d[i])
		if err != nil {
			return 0, err
		}
		newFee, err := math.Add(contribution, fee)
		if err != nil {
			return 0, err
		}
		fee = newFee
	}
	return fee, nil
}

func (f *Manager) UnitPrices() fees.Dimensions {
	f.l.RLock()
	defer f.l.RUnlock()

	var d fees.Dimensions
	for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
		d[i] = f.unitPrice(i)
	}
	return d
}

func (f *Manager) UnitsConsumed() fees.Dimensions {
	f.l.RLock()
	defer f.l.RUnlock()

	var d fees.Dimensions
	for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
		d[i] = f.lastConsumed(i)
	}
	return d
}

func computeNextPriceWindow(
	previous window.Window,
	previousConsumed uint64,
	previousPrice uint64,
	target uint64, /* per window */
	changeDenom uint64,
	minPrice uint64,
	since uint64, /* seconds */
) (uint64, window.Window) {
	newRollupWindow := window.Roll(previous, since)
	if since < window.WindowSize {
		// add in the units used by the parent block in the correct place
		// If the parent consumed units within the rollup window, add the consumed
		// units in.
		slot := window.WindowSize - 1 - int(since)
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
		n, over := math.Add(nextPrice, baseDelta)
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
			baseDelta *= since / window.WindowSize
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
	return nextPrice, newRollupWindow
}

type Rules interface {
	GetMinUnitPrice() fees.Dimensions
	GetUnitPriceChangeDenominator() fees.Dimensions
	GetWindowTargetUnits() fees.Dimensions
	GetMaxBlockUnits() fees.Dimensions
}

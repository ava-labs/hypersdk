// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/window"
)

func TestUnitsConsumed(t *testing.T) {
	tests := []struct {
		name           string
		initialUnits   fees.Dimensions
		unitsToConsume fees.Dimensions
		expectedUnits  fees.Dimensions
		maxUnits       fees.Dimensions
		success        bool
	}{
		{
			name:           "empty manager consumes max",
			unitsToConsume: fees.Dimensions{2, 2, 2, 2, 2},
			expectedUnits:  fees.Dimensions{2, 2, 2, 2, 2},
			maxUnits:       fees.Dimensions{2, 2, 2, 2, 2},
			success:        true,
		},
		{
			name:           "empty manager consumes less than max",
			unitsToConsume: fees.Dimensions{1, 1, 1, 1, 1},
			expectedUnits:  fees.Dimensions{1, 1, 1, 1, 1},
			maxUnits:       fees.Dimensions{2, 2, 2, 2, 2},
			success:        true,
		},
		{
			name:           "empty manager exceeds max of one dimension",
			unitsToConsume: fees.Dimensions{1, 2, 1, 1, 1},
			maxUnits:       fees.Dimensions{1, 1, 1, 1, 1},
			success:        false,
		},
		{
			name:           "empty manager exceeds max of all dimensions",
			unitsToConsume: fees.Dimensions{3, 3, 3, 3, 3},
			maxUnits:       fees.Dimensions{2, 2, 2, 2, 2},
			success:        false,
		},
		{
			name:           "non-empty manager consumes less than max",
			initialUnits:   fees.Dimensions{1, 1, 1, 1, 1},
			unitsToConsume: fees.Dimensions{1, 1, 1, 1, 1},
			expectedUnits:  fees.Dimensions{2, 2, 2, 2, 2},
			maxUnits:       fees.Dimensions{3, 3, 3, 3, 3},
			success:        true,
		},
		{
			name:           "non-empty manager consumes max of one dimension",
			initialUnits:   fees.Dimensions{1, 1, 1, 1, 1},
			unitsToConsume: fees.Dimensions{1, 2, 1, 1, 1},
			expectedUnits:  fees.Dimensions{2, 3, 2, 2, 2},
			maxUnits:       fees.Dimensions{3, 3, 3, 3, 3},
			success:        true,
		},
		{
			name:           "non-empty manager consumes max of all dimension",
			initialUnits:   fees.Dimensions{1, 1, 1, 1, 1},
			unitsToConsume: fees.Dimensions{2, 2, 2, 2, 2},
			expectedUnits:  fees.Dimensions{3, 3, 3, 3, 3},
			maxUnits:       fees.Dimensions{3, 3, 3, 3, 3},
			success:        true,
		},
		{
			name:           "non-empty manager exceeds max of one dimension",
			initialUnits:   fees.Dimensions{1, 1, 1, 1, 1},
			unitsToConsume: fees.Dimensions{1, 3, 1, 1, 1},
			maxUnits:       fees.Dimensions{3, 3, 3, 3, 3},
			success:        false,
		},
		{
			name:           "non-empty manager exceeds max of all dimension",
			initialUnits:   fees.Dimensions{1, 1, 1, 1, 1},
			unitsToConsume: fees.Dimensions{3, 3, 3, 3, 3},
			maxUnits:       fees.Dimensions{3, 3, 3, 3, 3},
			success:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			// Initializing manager
			manager := NewManager([]byte{})
			ok, _ := manager.Consume(tt.initialUnits, tt.maxUnits)
			r.True(ok)

			// Testing manager
			ok, _ = manager.Consume(tt.unitsToConsume, tt.maxUnits)
			r.Equal(tt.success, ok)
			if tt.success {
				r.Equal(tt.expectedUnits, manager.UnitsConsumed())
			}
		})
	}
}

// mulDiv must compute floor(a*b/d) exactly whenever the quotient is
// representable, even when the intermediate product exceeds 64 bits, and
// saturate rather than wrap or panic otherwise.
func TestMulDiv(t *testing.T) {
	maxU64 := ^uint64(0)

	tests := []struct {
		name    string
		a, b, d uint64
		want    uint64
	}{
		{
			name: "no overflow",
			a:    7, b: 6, d: 4,
			want: 10, // floor(42/4)
		},
		{
			name: "zero numerator",
			a:    0, b: 12345, d: 7,
			want: 0,
		},
		{
			// the fee-price case that used to wrap: the product is ~3.3e19 but
			// the quotient fits comfortably
			name: "product exceeds 64 bits, quotient fits",
			a:    359191338483402, b: 93000, d: 1000,
			want: 33404794478956386,
		},
		{
			name: "quotient does not fit, saturates",
			a:    1 << 63, b: 1 << 63, d: 1,
			want: maxU64,
		},
		{
			name: "max inputs",
			a:    maxU64, b: maxU64, d: maxU64,
			want: maxU64,
		},
		{
			name: "zero divisor saturates instead of panicking",
			a:    12345, b: 6789, d: 0,
			want: maxU64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, mulDiv(tt.a, tt.b, tt.d))
		})
	}
}

type testFeeRules struct {
	min, denom, target, max fees.Dimensions
}

func (r testFeeRules) GetMinUnitPrice() fees.Dimensions               { return r.min }
func (r testFeeRules) GetUnitPriceChangeDenominator() fees.Dimensions { return r.denom }
func (r testFeeRules) GetWindowTargetUnits() fees.Dimensions          { return r.target }
func (r testFeeRules) GetMaxBlockUnits() fees.Dimensions              { return r.max }

// Under sustained over-target load the unit price must rise monotonically until
// it saturates. Before mulDiv, previousPrice*delta wrapped once the price grew
// past roughly 2^44, which made the price update erratic and left congestion
// pricing under-responding by several multiples.
func TestComputeNextPriceNoOverflowUnderSustainedLoad(t *testing.T) {
	require := require.New(t)

	// hypersdk default rules (genesis.NewDefaultRules)
	r := testFeeRules{
		min:    fees.Dimensions{100, 100, 100, 100, 100},
		denom:  fees.Dimensions{48, 48, 48, 48, 48},
		target: fees.Dimensions{20_000_000, 1_000, 1_000, 1_000, 1_000},
		max:    fees.Dimensions{1_800_000, 2_000, 2_000, 2_000, 2_000},
	}
	const dim = fees.Compute

	m := NewManager(nil)
	for i := fees.Dimension(0); i < fees.FeeDimensions; i++ {
		m.SetUnitPrice(i, r.min[i])
	}

	// a block every MinBlockGap (100ms), each consuming the full block limit
	var (
		tsMs = int64(0)
		prev = m.UnitPrice(dim)
	)
	for block := 1; block <= 500; block++ {
		m.SetLastConsumed(dim, r.max[dim])
		tsMs += 100
		m = m.ComputeNext(tsMs, r)

		price := m.UnitPrice(dim)
		require.GreaterOrEqualf(price, prev,
			"price decreased at block %d (%d -> %d) under sustained over-target load", block, prev, price)
		prev = price
	}

	// sustained saturation should have driven the price to its ceiling
	require.Equal(^uint64(0), prev)
}

var refMaxU64 = new(big.Int).SetUint64(^uint64(0))

func clampToU64(v *big.Int) uint64 {
	if v.Cmp(refMaxU64) > 0 {
		return ^uint64(0)
	}
	if v.Sign() < 0 {
		return 0
	}
	return v.Uint64()
}

// refNextPrice models computeNextPriceWindow's price arithmetic exactly, using
// big.Int for the intermediate and saturating only where the implementation
// documents that it does.
func refNextPrice(prevPrice, total, target, changeDenom, minPrice, since uint64) uint64 {
	bi := func(v uint64) *big.Int { return new(big.Int).SetUint64(v) }

	baseDelta := func(delta uint64) uint64 {
		y := new(big.Int).Div(new(big.Int).Mul(bi(prevPrice), bi(delta)), bi(target))
		bd := clampToU64(y) / changeDenom
		if bd < 1 {
			bd = 1
		}
		return bd
	}

	next := prevPrice
	switch {
	case total > target:
		next = clampToU64(new(big.Int).Add(bi(prevPrice), bi(baseDelta(total-target))))
	case total < target:
		bd := baseDelta(target - total)
		if since > window.WindowSize {
			bd = clampToU64(new(big.Int).Mul(bi(bd), bi(since/window.WindowSize)))
		}
		next = clampToU64(new(big.Int).Sub(bi(prevPrice), bi(bd)))
	}
	if next < minPrice {
		next = minPrice
	}
	return next
}

// Differential test across both the increase and decrease branches, including
// the idle-decay scaling path. Guards against the intermediate product wrapping
// at 2^64, which previously made the price update diverge from its intent by
// several multiples once unit prices grew past roughly 2^44.
func TestComputeNextPriceWindowMatchesExactReference(t *testing.T) {
	require := require.New(t)
	rng := rand.New(rand.NewSource(20260820)) //nolint:gosec // deterministic test input

	var (
		prices  = []uint64{0, 1, 100, 1 << 20, 1 << 40, 1 << 44, 1 << 50, 1 << 62, ^uint64(0)}
		targets = []uint64{1, 1000, 20_000_000, 1 << 40}
		denoms  = []uint64{1, 2, 48, 1000}
		sinces  = []uint64{0, 1, 5, 9, 10, 11, 100, 10_000, 1 << 40}
	)

	const minPrice = 100
	var increase, decrease int

	for _, prevPrice := range prices {
		for _, target := range targets {
			for _, denom := range denoms {
				for _, since := range sinces {
					for trial := 0; trial < 6; trial++ {
						var w window.Window
						for i := 0; i < window.WindowSize; i++ {
							var v uint64
							switch trial {
							case 0:
								v = 0
							case 1:
								v = target / uint64(window.WindowSize)
							case 2:
								v = ^uint64(0) / uint64(window.WindowSize)
							default:
								v = rng.Uint64() % (2*target + 1)
							}
							window.Update(&w, i*8, v)
						}
						prevConsumed := rng.Uint64() % (2*target + 1)

						gotPrice, gotWindow := computeNextPriceWindow(
							w, prevConsumed, prevPrice, target, denom, minPrice, since,
						)

						// window.Roll/Update/Sum are unchanged by this fix, so
						// deriving the observed total with them is sound
						total := window.Sum(gotWindow)
						switch {
						case total > target:
							increase++
						case total < target:
							decrease++
						}

						require.Equalf(
							refNextPrice(prevPrice, total, target, denom, minPrice, since),
							gotPrice,
							"prevPrice=%d total=%d target=%d denom=%d since=%d",
							prevPrice, total, target, denom, since,
						)
					}
				}
			}
		}
	}

	// make sure the table actually exercised both branches
	require.Positive(increase)
	require.Positive(decrease)
}

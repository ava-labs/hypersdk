// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ava-labs/hypersdk/chain"
)

var (
	_ VM = (*FakeVM)(nil)
)

type FakeVM struct {
	rules chain.Rules
}

func (_ *FakeVM) StopChan() chan struct{} {
	return nil
}

func (_ *FakeVM) EngineChan() chan<- common.Message {
	return nil
}

func (_ *FakeVM) PreferredBlock(context.Context) (*chain.StatelessBlock, error) {
	return nil, nil
}

func (_ *FakeVM) Logger() logging.Logger {
	return nil
}

func (_ *FakeVM) Mempool() chain.Mempool {
	return nil
}

func (f *FakeVM) Rules(int64) chain.Rules {
	return f.rules
}

func TestNextTime(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name        string
		now         int64
		lastQueue   int64
		minBlockGap int64
		preferred   int64
		expected    int64
	}{
		{
			name:     "uninitialized is minBuildGap",
			expected: minBuildGap,
		},
		{
			name:        "nextTime is lastQueue plus minBuildGap",
			lastQueue:   100,
			preferred:   110,
			minBlockGap: 14,
			expected:    125,
		},
		{
			name:        "nextTime is preferred time plus gap",
			now:         1000,
			lastQueue:   800,
			preferred:   1500,
			minBlockGap: 200,
			expected:    1700,
		},
		{
			name:        "nextTime is in the past",
			now:         2000,
			lastQueue:   1400,
			preferred:   1500,
			minBlockGap: 200,
			expected:    -1,
		},
		{
			name:        "nextTime is exactly now",
			now:         2000,
			lastQueue:   1500,
			preferred:   1800,
			minBlockGap: 200,
			expected:    2000,
		},
		{
			name:        "nextTime is higher than now",
			now:         500,
			lastQueue:   300,
			preferred:   600,
			minBlockGap: 100,
			expected:    700,
		},
		{
			name:        "preferred is far in the future",
			now:         1000,
			lastQueue:   900,
			preferred:   5000,
			minBlockGap: 10,
			expected:    5010,
		},
		{
			name:        "minBlockGap upgrade cannot return",
			now:         1000,
			lastQueue:   900,
			preferred:   5000,
			minBlockGap: 10,
			expected:    5010,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			rules := chain.NewMockRules(ctrl)
			rules.EXPECT().GetMinBlockGap().Return(tt.minBlockGap)

			vm := &FakeVM{
				rules,
			}
			time := NewTime(vm)

			time.lastQueue = tt.lastQueue

			next := time.nextTime(tt.now, tt.preferred)
			require.Equal(tt.expected, next)
		})
	}
}

func TestNextTimeIncreases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name           string
		now            int64
		lastQueue      int64
		minBlockGap    int64
		newMinBlockGap int64
		preferred      int64
		newPreferred   int64
		expected       int64
	}{
		{
			name:           "minBlockGap increase",
			now:            1000,
			lastQueue:      900,
			minBlockGap:    25,
			newMinBlockGap: 50,
			preferred:      1100,
			newPreferred:   1200,
			expected:       1250,
		},
		{
			name:           "minBlockGap decrease",
			now:            1000,
			lastQueue:      950,
			minBlockGap:    50,
			newMinBlockGap: 25,
			preferred:      1100,
			newPreferred:   1200,
			expected:       1225,
		},
		{
			name:           "nextTime is higher than now after gap increase",
			now:            500,
			lastQueue:      300,
			minBlockGap:    25,
			newMinBlockGap: 100,
			preferred:      600,
			newPreferred:   650,
			expected:       750,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			rules := chain.NewMockRules(ctrl)
			rules.EXPECT().GetMinBlockGap().Return(tt.minBlockGap)

			vm := &FakeVM{
				rules,
			}
			time := NewTime(vm)

			time.lastQueue = tt.lastQueue

			firstNext := time.nextTime(tt.now, tt.preferred)
			time.lastQueue = firstNext

			rules.EXPECT().GetMinBlockGap().Return(tt.newMinBlockGap)

			next := time.nextTime(firstNext, tt.newPreferred)
			require.Equal(tt.expected, next)
			require.GreaterOrEqual(next, firstNext)
		})
	}
}

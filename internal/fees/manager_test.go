// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/fees"
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

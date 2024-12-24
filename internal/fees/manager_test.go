// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"testing"

	"github.com/stretchr/testify/require"

	externalFees "github.com/ava-labs/hypersdk/fees"
)

func TestUnitsConsumed(t *testing.T) {
	tests := []struct {
		name           string
		initialUnits   externalFees.Dimensions
		unitsToConsume externalFees.Dimensions
		maxUnits       externalFees.Dimensions
		success        bool
	}{
		{
			name:           "empty manager consumes",
			unitsToConsume: externalFees.Dimensions{1, 2, 3, 4, 5},
			maxUnits:       externalFees.Dimensions{1, 2, 3, 4, 5},
			success:        true,
		},
		{
			name:           "nonempty manager consumes",
			initialUnits:   externalFees.Dimensions{1, 2, 3, 4, 5},
			unitsToConsume: externalFees.Dimensions{1, 2, 3, 4, 5},
			maxUnits:       externalFees.Dimensions{2, 4, 6, 8, 10},
			success:        true,
		},
		{
			name:           "consume fails if consumed > available",
			initialUnits:   externalFees.Dimensions{1, 2, 3, 4, 5},
			unitsToConsume: externalFees.Dimensions{1, 2, 3, 4, 5},
			maxUnits:       externalFees.Dimensions{1, 2, 3, 4, 5},
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
				r.Equal(tt.maxUnits, manager.UnitsConsumed())
			}
		})
	}
}

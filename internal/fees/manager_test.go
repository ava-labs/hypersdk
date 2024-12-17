// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"testing"

	"github.com/stretchr/testify/require"

	externalFees "github.com/ava-labs/hypersdk/fees"
)

func TestUnitsConsumed(t *testing.T) {
	t.Run("empty manager consumes", func(t *testing.T) {
		r := require.New(t)

		manager := NewManager([]byte{})
		units := externalFees.Dimensions{1, 2, 3, 4, 5}

		ok, _ := manager.Consume(units, units)
		r.True(ok)
		r.Equal(units, manager.UnitsConsumed())
	})

	t.Run("nonempty manager consumes", func(t *testing.T) {
		r := require.New(t)

		manager := NewManager([]byte{})
		initialUnits := externalFees.Dimensions{1, 2, 3, 4, 5}
		finalUnits := externalFees.Dimensions{2, 4, 6, 8, 10}

		ok, _ := manager.Consume(initialUnits, initialUnits)
		r.True(ok)

		ok, _ = manager.Consume(initialUnits, finalUnits)
		r.True(ok)

		r.Equal(finalUnits, manager.UnitsConsumed())
	})

	t.Run("consume fails if consumed > available", func(t *testing.T) {
		r := require.New(t)

		manager := NewManager([]byte{})
		initialUnits := externalFees.Dimensions{1, 2, 3, 4, 5}

		ok, _ := manager.Consume(initialUnits, initialUnits)
		r.True(ok)

		ok, dim := manager.Consume(initialUnits, initialUnits)
		r.False(ok)
		r.Equal(externalFees.Dimension(0), dim)
	})
}

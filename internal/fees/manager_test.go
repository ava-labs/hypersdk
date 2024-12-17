// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	externalfees "github.com/ava-labs/hypersdk/fees"
)

func TestUnitsConsumedFuzz(t *testing.T) {
	r := require.New(t)
	rand := rand.New(rand.NewSource(0)) //nolint:gosec

	for fuzzIteration := 0; fuzzIteration < 1000; fuzzIteration++ {
		m := NewManager([]byte{})
		dims := externalfees.Dimensions{}

		for i := range dims {
			dims[i] = rand.Uint64()
			m.SetLastConsumed(externalfees.Dimension(i), dims[i])
		}

		r.Equal(dims, m.UnitsConsumed())
	}
}

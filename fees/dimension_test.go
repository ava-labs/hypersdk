// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDimensionsMarshalText(t *testing.T) {
	require := require.New(t)
	var dim Dimensions
	dim[Bandwidth] = 1
	dim[Compute] = 2
	dim[StorageRead] = 3
	dim[StorageAllocate] = 4
	dim[StorageWrite] = 5

	dimTextBytes, err := dim.MarshalText()
	require.NoError(err)

	var parsedDimText Dimensions
	require.NoError(parsedDimText.UnmarshalText(dimTextBytes))
	require.Equal(dim, parsedDimText)
}

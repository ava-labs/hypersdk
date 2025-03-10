// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsedABIs(t *testing.T) {
	r := require.New(t)

	_, err := ParseABIs("../abis")
	r.NoError(err)
}

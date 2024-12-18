// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"testing"

	"github.com/stretchr/testify/require"

	externalfees "github.com/ava-labs/hypersdk/fees"
)

func TestManager(t *testing.T) {
	r := require.New(t)
	v := uint64(1)

	m := NewManager([]byte{})
	m.SetLastConsumed(externalfees.Dimension(2), v)

	r.Equal(v, m.LastConsumed(externalfees.Dimension(2)))
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultSerialization(t *testing.T) {
	require := require.New(t)
	// marshal default builder
	builderDefault := NewConfigBuilder()
	defaultBytes, err := json.Marshal(builderDefault)
	require.NoError(err)
	// unmarshal and ensure defaults
	builder := &ConfigBuilder{}
	err = json.Unmarshal(defaultBytes, builder)
	require.NoError(err)
	require.Equal(builderDefault, builder)
}

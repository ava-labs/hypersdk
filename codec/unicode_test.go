// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnicodeBytes(t *testing.T) {
	require := require.New(t)

	original := UnicodeBytes("Hello, 世界!")

	marshalledBytes, err := original.MarshalText()
	require.NoError(err)
	require.Equal([]byte("Hello, 世界!"), marshalledBytes)

	var unmarshalled UnicodeBytes
	err = unmarshalled.UnmarshalText(marshalledBytes)
	require.NoError(err)
	require.Equal(original, unmarshalled)

	jsonBytes, err := json.Marshal(original)
	require.NoError(err)
	require.Equal([]byte(`"Hello, 世界!"`), jsonBytes)

	var jsonUnmarshalled UnicodeBytes
	err = json.Unmarshal(jsonBytes, &jsonUnmarshalled)
	require.NoError(err)
	require.Equal(original, jsonUnmarshalled)
}

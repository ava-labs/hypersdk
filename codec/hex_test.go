// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBytesHex(t *testing.T) {
	require := require.New(t)
	b := []byte{1, 2, 3, 4, 5}
	wrappedBytes := Bytes(b)

	marshalledBytes, err := wrappedBytes.MarshalText()
	require.NoError(err)

	var unmarshalledBytes Bytes
	require.NoError(unmarshalledBytes.UnmarshalText(marshalledBytes))
	require.Equal(b, []byte(unmarshalledBytes))

	jsonMarshalledBytes, err := json.Marshal(wrappedBytes)
	require.NoError(err)

	var jsonUnmarshalledBytes Bytes
	require.NoError(json.Unmarshal(jsonMarshalledBytes, &jsonUnmarshalledBytes))
	require.Equal(b, []byte(jsonUnmarshalledBytes))
}

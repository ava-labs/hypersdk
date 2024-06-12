// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSerializationRawBytes(t *testing.T) {
	require := require.New(t)
	testBytes := RawBytes([]byte{0, 1, 2, 3})
	serializedBytes, err := serialize(testBytes)
	require.NoError(err)
	require.Equal(([]byte)(testBytes), serializedBytes)

	serializedBytes, err = testBytes.customSerialize()
	require.NoError(err)
	require.Equal(([]byte)(testBytes), serializedBytes)

	deserialized, err := RawBytes{}.customDeserialize(serializedBytes)
	require.NoError(err)
	require.Equal(testBytes, *deserialized)

	deserialized, err = deserialize[RawBytes](serializedBytes)
	require.NoError(err)
	require.Equal(testBytes, *deserialized)
}

func TestSerializationResult(t *testing.T) {
	require := require.New(t)
	testResult := Ok[byte, byte](1)

	serializedBytes, err := serialize(testResult)
	require.NoError(err)
	require.Equal([]byte{1, 1}, serializedBytes)

	serializedBytes, err = testResult.customSerialize()
	require.NoError(err)
	require.Equal([]byte{1, 1}, serializedBytes)

	deserialized, err := Result[byte, byte]{}.customDeserialize(serializedBytes)
	require.NoError(err)
	require.Equal(testResult, *deserialized)

	deserialized, err = deserialize[Result[byte, byte]](serializedBytes)
	require.NoError(err)
	require.Equal(testResult, *deserialized)
}

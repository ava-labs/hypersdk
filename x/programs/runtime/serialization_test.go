// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSerializationRawBytes(t *testing.T) {
	require := require.New(t)
	testBytes := RawBytes([]byte{0, 1, 2, 3})
	serializedBytes, err := serialize(testBytes)
	require.NoError(err)
	require.Equal(([]byte)(testBytes), serializedBytes)

	b := new(bytes.Buffer)
	err = testBytes.customSerialize(b)
	require.NoError(err)
	require.Equal(([]byte)(testBytes), b.Bytes())

	deserialized, err := RawBytes{}.customDeserialize(b.Bytes())
	require.NoError(err)
	require.Equal(testBytes, *deserialized)

	deserialized, err = deserialize[RawBytes](b.Bytes())
	require.NoError(err)
	require.Equal(testBytes, *deserialized)
}

func TestSerializationResult(t *testing.T) {
	require := require.New(t)
	testResult := Ok[byte, byte](1)

	serializedBytes, err := serialize(testResult)
	require.NoError(err)
	require.Equal([]byte{1, 1}, serializedBytes)

	b := new(bytes.Buffer)
	err = testResult.customSerialize(b)
	require.NoError(err)
	require.Equal([]byte{1, 1}, b.Bytes())

	deserialized, err := Result[byte, byte]{}.customDeserialize(b.Bytes())
	require.NoError(err)
	require.Equal(testResult, *deserialized)

	deserialized, err = deserialize[Result[byte, byte]](b.Bytes())
	require.NoError(err)
	require.Equal(testResult, *deserialized)
}

func TestSerializationOption(t *testing.T) {
	require := require.New(t)
	testOption := Some[byte](1)

	serializedBytes, err := serialize(testOption)
	require.NoError(err)
	require.Equal([]byte{optionSomePrefix, 1}, serializedBytes)

	b := new(bytes.Buffer)
	err = testOption.customSerialize(b)
	require.NoError(err)
	require.Equal([]byte{optionSomePrefix, 1}, b.Bytes())

	deserialized, err := Option[byte]{}.customDeserialize(b.Bytes())
	require.NoError(err)
	require.Equal(testOption, *deserialized)

	deserialized, err = deserialize[Option[byte]](b.Bytes())
	require.NoError(err)
	require.Equal(testOption, *deserialized)
}

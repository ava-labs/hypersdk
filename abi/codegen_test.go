// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"encoding/json"
	"go/format"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAllStructs(t *testing.T) {
	require := require.New(t)

	abi := mustJSONParse[VMABI](t, string(mustReadFile(t, "testdata/abi.json")))

	code, err := GenerateGoStructs(abi, "testdata")
	require.NoError(err)

	expected := mustReadFile(t, "testdata/abi.go")

	formatted, err := format.Source([]byte(string(expected)))
	require.NoError(err)

	require.Equal(string(formatted), code)
}

func mustJSONParse[T any](t *testing.T, jsonStr string) T {
	var parsed T
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err, jsonStr)
	return parsed
}

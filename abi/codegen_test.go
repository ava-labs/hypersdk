package abi

import (
	"encoding/json"
	"go/format"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAllStructs(t *testing.T) {
	require := require.New(t)

	abi := mustJSONParse[VMABI](t, string(mustReadFile(t, "test_data/abi.json")))

	code, err := GenerateGoStructs(abi, "test_data")
	require.NoError(err)

	expected := mustReadFile(t, "test_data/abi.go")

	require.Equal(mustFormat(t, string(expected)), code)
}

func mustFormat(t *testing.T, code string) string {
	formatted, err := format.Source([]byte(code))
	require.NoError(t, err)

	return string(formatted)
}

func mustJSONParse[T any](t *testing.T, jsonStr string) T {
	var parsed T
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err, jsonStr)
	return parsed
}

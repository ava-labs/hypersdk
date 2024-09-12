package abi

import (
	"encoding/json"
	"go/format"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAllStructs(t *testing.T) {
	require := require.New(t)

	abi := mustJSONParse[ABI](t, string(mustReadFile(t, "testdata/abi.json")))

	code, err := GenerateGoStructs(abi, "abi")
	require.NoError(err)

	expected := mustReadFile(t, "mockabi.test.go")

	formatted, err := format.Source(removeCommentLines(expected))
	require.NoError(err)

	require.Equal(string(formatted), code)
}

func removeCommentLines(input []byte) []byte {
	lines := strings.Split(string(input), "\n")
	var result []string
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			result = append(result, line)
		}
	}
	return []byte(strings.Join(result, "\n"))
}

func mustJSONParse[T any](t *testing.T, jsonStr string) T {
	var parsed T
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err, jsonStr)
	return parsed
}

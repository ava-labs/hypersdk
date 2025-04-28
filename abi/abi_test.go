// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
)

func TestDummy(t *testing.T) {
	r := require.New(t)

	actionParser := codec.NewCanotoParser[chain.Action]()
	outputParser := codec.NewCanotoParser[codec.Typed]()

	r.NoError(actionParser.Register(&chaintest.TestAction{}, chaintest.UnmarshalTestAction))
	r.NoError(outputParser.Register(&chaintest.TestOutput{}, chaintest.UnmarshalTestOutput))

	expectedABI := NewABI(actionParser, outputParser)

	bytes := mustReadFile(t, "testdata/abi.json")

	actualABI := ABI{}
	r.NoError(json.Unmarshal(bytes, &actualABI))
	actualABI.CalculateCanotoSpec()

	r.Equal(expectedABI, actualABI)
}

// func TestNewABI(t *testing.T) {
// 	require := require.New(t)

// 	actionParser := codec.NewCanotoParser[chain.Action]()
// 	outputParser := codec.NewCanotoParser[codec.Typed]()

// 	actualABI := NewTempABI(actionParser, outputParser)

// 	actualABI, err := NewABI([]codec.Typed{
// 		MockObjectSingleNumber{},
// 		MockActionTransfer{},
// 		MockObjectAllNumbers{},
// 		MockObjectStringAndBytes{},
// 		MockObjectArrays{},
// 		MockActionWithTransfer{},
// 		MockActionWithTransferArray{},
// 		Outer{},
// 		ActionWithOutput{},
// 		FixedBytes{},
// 		Bools{},
// 	}, []codec.Typed{
// 		ActionOutput{},
// 	})
// 	require.NoError(err)

// 	expectedABIJSON := mustReadFile(t, "testdata/abi.json")
// 	expectedABI := mustJSONParse[ABI](t, string(expectedABIJSON))

// 	require.Equal(expectedABI, actualABI)
// }

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

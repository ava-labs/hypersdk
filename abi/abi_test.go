// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/StephenButtolph/canoto"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
)

func TestABIHash(t *testing.T) {
	r := require.New(t)

	abiJSON := mustReadFile(t, "testdata/abi.json")
	abi := &ABI{}
	r.NoError(json.Unmarshal(abiJSON, abi))

	abiBytes := abi.MarshalCanoto()

	abiHash := sha256.Sum256(abiBytes)
	expectedHashHex := strings.TrimSpace(string(mustReadFile(t, "testdata/abi.hash.hex")))
	r.Equal(expectedHashHex, hex.EncodeToString(abiHash[:]))
}

func TestABIFindActionSpecByName(t *testing.T) {
	tests := []struct {
		name       string
		actionName string
		spec       *canoto.Spec
		exists     bool
	}{
		{
			name:       "registered action",
			actionName: "TestAction",
			spec:       (&chaintest.TestAction{}).CanotoSpec(),
			exists:     true,
		},
		{
			name:       "unregistered action",
			actionName: "NotTestAction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			actionParser := codec.NewCanotoParser[chain.Action]()
			outputParser := codec.NewCanotoParser[codec.Typed]()

			r.NoError(actionParser.Register(&chaintest.TestAction{}, chaintest.UnmarshalTestAction))

			abi := NewABI(actionParser, outputParser)
			abi.CalculateCanotoSpec()

			spec, exists := abi.FindActionSpecByName(tt.actionName)
			r.Equal(tt.exists, exists)
			if exists {
				r.Equal(tt.spec, spec)
			}
		})
	}
}

func TestABIFindOutputSpecByID(t *testing.T) {
	tests := []struct {
		name     string
		outputID uint8
		spec     *canoto.Spec
		exists   bool
	}{
		{
			name:     "registered output",
			outputID: (&chaintest.TestOutput{}).GetTypeID(),
			spec:     (&chaintest.TestOutput{}).CanotoSpec(),
			exists:   true,
		},
		{
			name:     "unregistered output",
			outputID: (&chaintest.TestOutput{}).GetTypeID() + 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			actionParser := codec.NewCanotoParser[chain.Action]()
			outputParser := codec.NewCanotoParser[codec.Typed]()

			r.NoError(outputParser.Register(&chaintest.TestOutput{}, chaintest.UnmarshalTestOutput))

			abi := NewABI(actionParser, outputParser)
			abi.CalculateCanotoSpec()

			spec, exists := abi.FindOutputSpecByID(tt.outputID)
			r.Equal(tt.exists, exists)
			if exists {
				r.Equal(tt.spec, spec)
			}
		})
	}
}

func TestABIGetActionID(t *testing.T) {
	tests := []struct {
		name       string
		actionName string
		typeID     uint8
		exists     bool
	}{
		{
			name:       "registered action",
			actionName: "TestAction",
			typeID:     (&chaintest.TestAction{}).GetTypeID(),
			exists:     true,
		},
		{
			name:       "unregistered action",
			actionName: "NotTestAction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			actionParser := codec.NewCanotoParser[chain.Action]()
			outputParser := codec.NewCanotoParser[codec.Typed]()

			r.NoError(actionParser.Register(&chaintest.TestAction{}, chaintest.UnmarshalTestAction))

			abi := NewABI(actionParser, outputParser)
			abi.CalculateCanotoSpec()

			actionID, found := abi.GetActionID(tt.actionName)
			r.Equal(tt.exists, found)
			if tt.exists {
				r.Equal(tt.typeID, actionID)
			}
		})
	}
}

func TestABIGetOutputID(t *testing.T) {
	tests := []struct {
		name       string
		outputName string
		typeID     uint8
		exists     bool
	}{
		{
			name:       "registered output",
			outputName: "TestOutput",
			typeID:     (&chaintest.TestOutput{}).GetTypeID(),
			exists:     true,
		},
		{
			name:       "unregistered output",
			outputName: "NotTestOutput",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			actionParser := codec.NewCanotoParser[chain.Action]()
			outputParser := codec.NewCanotoParser[codec.Typed]()

			r.NoError(outputParser.Register(&chaintest.TestOutput{}, chaintest.UnmarshalTestOutput))

			abi := NewABI(actionParser, outputParser)
			abi.CalculateCanotoSpec()

			outputID, found := abi.GetOutputID(tt.outputName)
			r.Equal(tt.exists, found)
			if tt.exists {
				r.Equal(tt.typeID, outputID)
			}
		})
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

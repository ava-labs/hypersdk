// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
)

func TestResultJSON(t *testing.T) {
	require := require.New(t)
	errStrBytes := []byte("error")
	outputBytes := []byte("output")
	units := fees.Dimensions{1, 2, 3, 4, 5}
	unitsJSON, err := json.Marshal(units)
	require.NoError(err)
	result := &Result{
		Success: true,
		Error:   errStrBytes,
		Outputs: [][]byte{outputBytes},
		Units:   units,
		Fee:     4,
	}

	resultJSON, err := json.Marshal(result)
	require.NoError(err)

	expectedJSON := fmt.Sprintf(`{"error":%q,"fee":4,"outputs":[%q],"success":true,"units":%s}`, codec.Bytes(errStrBytes), codec.Bytes(outputBytes), string(unitsJSON))
	require.JSONEq(expectedJSON, string(resultJSON))

	unmarshalledResult := new(Result)
	require.NoError(json.Unmarshal(resultJSON, unmarshalledResult))
	require.Equal(result, unmarshalledResult)
}

func TestResultMarshalIdentity(t *testing.T) {
	r := require.New(t)

	result := &Result{
		Success: true,
		Error:   []byte("error"),
		Outputs: [][]byte{[]byte("output")},
		Units:   fees.Dimensions{1, 2, 3, 4, 5},
		Fee:     4,
	}

	resultBytes := result.Marshal()
	unmarshalledResult, err := UnmarshalResult(resultBytes)
	r.NoError(err)
	r.Equal(result, unmarshalledResult)
}

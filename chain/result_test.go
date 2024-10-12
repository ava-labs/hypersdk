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
	result := Result{
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

	var unmarshalledResult Result
	require.NoError(json.Unmarshal(resultJSON, &unmarshalledResult))
	require.Equal(result, unmarshalledResult)
}

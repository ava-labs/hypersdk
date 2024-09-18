// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

func TestAddress(t *testing.T) {
	require := require.New(t)
	typeID := byte(0)
	addrID := ids.GenerateTestID()

	addr := CreateAddress(typeID, addrID)
	addrStr, err := addr.MarshalText()
	require.NoError(err)

	var parsedAddr Address
	require.NoError(parsedAddr.UnmarshalText([]byte(addrStr)))
	require.Equal(addr, parsedAddr)
}

func TestAddressJSON(t *testing.T) {
	require := require.New(t)
	typeID := byte(0)
	addrID := ids.GenerateTestID()

	addr := CreateAddress(typeID, addrID)

	addrJSONBytes, err := json.Marshal(addr)
	require.NoError(err)

	var parsedAddr Address
	require.NoError(json.Unmarshal(addrJSONBytes, &parsedAddr))
	require.Equal(addr, parsedAddr)
}

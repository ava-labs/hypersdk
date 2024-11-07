// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
)

func TestEncodingValidate(t *testing.T) {
	tests := []struct {
		input  Encoding
		exp    Encoding
		expErr error
	}{
		{
			input: JSON,
			exp:   JSON,
		},
		{
			input: Hex,
			exp:   Hex,
		},
		{
			input: Encoding(""),
			exp:   JSON,
		},
		{
			input:  Encoding("another"),
			expErr: ErrInvalidEncodingParameter,
		},
	}

	for _, test := range tests {
		err := test.input.Validate()
		if test.expErr != nil {
			require.EqualError(t, ErrInvalidEncodingParameter, err.Error())
		} else {
			require.Equal(t, test.exp, test.input)
		}
	}
}

func TestBlockResponse(t *testing.T) {
	blocks := chaintest.GenerateEmptyExecutedBlocks(require.New(t), ids.GenerateTestID(), 0, 0, 0, 1)
	t.Run("JSON", func(t *testing.T) {
		require := require.New(t)
		r := GetBlockResponse{}
		err := r.setResponse(blocks[0], JSON)
		require.NoError(err)
		marshaledBlock, err := json.Marshal(blocks[0])
		require.NoError(err)
		res, err := json.Marshal(r)
		require.NoError(err)
		require.Equal([]byte(fmt.Sprintf(`{"block":%s}`, string(marshaledBlock))), res)
	})
	t.Run("Hex", func(t *testing.T) {
		require := require.New(t)
		r := GetBlockResponse{}
		err := r.setResponse(blocks[0], Hex)
		require.NoError(err)
		blockBytes, err := blocks[0].Marshal()
		require.NoError(err)
		res, err := json.Marshal(r)
		require.NoError(err)
		marshaledBlock, err := codec.Bytes(blockBytes).MarshalText()
		require.NoError(err)
		require.Equal([]byte(fmt.Sprintf(`{"block":"%s"}`, string(marshaledBlock))), res)
	})
	t.Run("Unknown", func(t *testing.T) {
		require := require.New(t)
		r := GetBlockResponse{}
		err := r.setResponse(blocks[0], Encoding("unknown"))
		require.EqualError(err, ErrUnsupportedEncoding.Error())
	})
}

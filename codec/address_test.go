package codec

import (
	"bytes"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

const hrp = "blah"

func TestIDAddress(t *testing.T) {
	require := require.New(t)

	id := ids.GenerateTestID()
	addr, err := Address(hrp, id[:])
	require.NoError(err)

	sb, err := ParseAddress(hrp, addr, ids.IDLen)
	require.NoError(err)
	require.True(bytes.Equal(id[:], sb[:]))
}

func TestInvalidAddressChecksum(t *testing.T) {
	require := require.New(t)
	addr := "blah1859dz2uwazfgahey3j53ef2kqrans0c8cv4l78tda3rjkfw0txns8u2e7k"

	_, err := ParseAddress(hrp, addr, ids.IDLen)
	require.Error(err)
}

func TestMaxAddress(t *testing.T) {
	require := require.New(t)

	b := make([]byte, 49)
	b[0] = 1
	b[49-1] = 1
	addr, err := Address(hrp, b)
	require.NoError(err)

	sb, err := ParseAddress(hrp, addr, 49)
	require.NoError(err)
	require.True(bytes.Equal(b[:], sb[:]))
}

func TestCreateLargeAddress(t *testing.T) {
	require := require.New(t)

	b := make([]byte, 50)
	b[0] = 1
	b[50-1] = 1
	_, err := Address(hrp, b)
	require.ErrorIs(err, ErrInvalidSize)
}

func TestParseLargeAddress(t *testing.T) {
	require := require.New(t)

	addr := "blah1859dz2uwazfgahey3j53ef2kqrans0c8cv4l78tda3rjkfw0txnxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxs8u2e7k"
	_, err := ParseAddress(hrp, addr, 60)
	require.ErrorContains(err, "invalid bech32 string length")
}

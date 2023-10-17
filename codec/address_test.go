package codec

import (
	"bytes"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

const hrp = "blah"

func TestBasicAddress(t *testing.T) {
	require := require.New(t)

	id := ids.GenerateTestID()
	addr, err := Address(hrp, id[:])
	require.NoError(err)

	sb, err := ParseAddress(hrp, addr, ids.IDLen)
	require.NoError(err)
	require.True(bytes.Equal(id[:], sb[:]))
}

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrefix(t *testing.T) {
	require := require.New(t)
	stateKey := accountDataKey([]byte{0}, []byte{1, 2, 3})
	require.Equal([]byte{accountPrefix, 0, accountDataPrefix, 1, 2, 3}, stateKey)
}

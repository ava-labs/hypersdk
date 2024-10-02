package dsmr

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

var _ Tx = (*testTx)(nil)

func TestAddTx(t *testing.T) {
	require := require.New(t)

	n := New[*testTx](1)
	tx := &testTx{}
	chunk, err := n.Add(tx, 123)
	require.NoError(err)
	require.Contains(chunk.Items, tx)
}

type testTx struct{}

func (t testTx) GetID() ids.ID {
	return ids.Empty
}

func (t testTx) GetExpiry() time.Time {
	return time.Time{}
}

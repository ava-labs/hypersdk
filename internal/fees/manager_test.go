package fees

import (
	"testing"

	externalfees "github.com/ava-labs/hypersdk/fees"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	r := require.New(t)
	v := uint64(1)

	m := NewManager([]byte{})
	m.SetLastConsumed(externalfees.Dimension(2), v)

	r.Equal(v, m.LastConsumed(externalfees.Dimension(2)))
}

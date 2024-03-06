package flatdb

import (
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/stretchr/testify/require"
)

func TestFlatDB(t *testing.T) {
	require := require.New(t)
	db := New(t.TempDir(), true, 1024, 2*units.MiB)

	v, err := db.Get("test", "1")
	require.ErrorIs(err, database.ErrNotFound)
	require.Empty(v)

	require.NoError(db.Put("test", "1", []byte("2")))

	v, err = db.Get("test", "1")
	require.NoError(err)
	require.Equal([]byte("2"), v)

	require.NoError(db.Put("test", "2", []byte("3")))

	v, err = db.Get("test", "2")
	require.Nil(err)
	require.Equal([]byte("3"), v)

	require.NoError(db.Remove("test"))

	// Cache still works
	v, err = db.Get("test", "1")
	require.NoError(err)
	require.Equal([]byte("2"), v)
	v, err = db.Get("test", "2")
	require.Nil(err)
	require.Equal([]byte("3"), v)

	// Clear cache
	db.fileCache.Flush()
	v, err = db.Get("test", "1")
	require.ErrorIs(err, database.ErrNotFound)
	require.Empty(v)
	v, err = db.Get("test", "2")
	require.ErrorIs(err, database.ErrNotFound)
	require.Empty(v)
}

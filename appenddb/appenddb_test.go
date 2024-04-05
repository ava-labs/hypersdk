package appenddb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAppendDB(t *testing.T) {
	// Prepare
	require := require.New(t)
	baseDir := t.TempDir()
	logger := logging.NewLogger(
		"appenddb",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stdout,
			logging.Colors.ConsoleEncoder(),
		),
	)
	logger.Info("created directory", zap.String("path", baseDir))

	// Create
	db, last, err := New(logger, baseDir, 100)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Put
	b, err := db.NewBatch(10)
	require.NoError(err)
	b.Prepare()
	b.Put("hello", []byte("world"))
	batch, err := b.Write()
	require.NoError(err)
	require.NotEqual(ids.Empty, batch)

	// Get
	v, err := db.Get("hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)

	// Restart
	require.NoError(db.Close())
	db, last, err = New(logger, baseDir, 100)
	require.NoError(err)
	require.Equal(batch, last)

	// Get
	v, err = db.Get("hello")
	require.NoError(err)
	require.Equal([]byte("world"), v)

	// Corrupt file
	require.NoError(db.Close())
	f, err := os.OpenFile(filepath.Join(baseDir, "0"), os.O_RDWR, os.ModeAppend)
	require.NoError(err)
	_, err = f.WriteString("corrupt")
	require.NoError(err)
	require.NoError(f.Close())

	// Restart
	db, last, err = New(logger, baseDir, 100)
	require.NoError(err)
	require.Equal(ids.Empty, last)

	// Get
	v, err = db.Get("hello")
	require.ErrorIs(err, database.ErrNotFound)
	require.NoError(db.Close())
}

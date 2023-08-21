package chain

import (
	"context"
)

var _ Database = (*ReadOnlyDatabase)(nil)

type ReadOnlyDatabase struct {
	Database
}

func NewReadOnlyDatabase(db Database) *ReadOnlyDatabase {
	return &ReadOnlyDatabase{db}
}

func (*ReadOnlyDatabase) Insert(_ context.Context, _ []byte, _ []byte) error {
	return ErrModificationNotAllowed
}

func (*ReadOnlyDatabase) Remove(_ context.Context, _ []byte) error {
	return ErrModificationNotAllowed
}

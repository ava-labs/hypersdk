package runtime

import (
	"context"
)

type Runtime interface {
	Initialize(context.Context, []byte, []string) error
	Call(context.Context, string, ...uint64) ([]uint64, error)
	GetGuestBuffer(uint32, uint32) ([]byte, bool)
	WriteGuestBuffer(context.Context, []byte) (uint64, error)
	Stop(context.Context) error
}

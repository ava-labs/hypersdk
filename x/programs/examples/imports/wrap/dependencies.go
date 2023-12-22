package wrap

import (
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/ava-labs/hypersdk/x/programs/program/types"
)

type OneParam interface {
	Call(*program.Caller, int64) (*types.Val, error)
}

type TwoParam interface {
	Call(*program.Caller, int64, int64) (*types.Val, error)
}

type ThreeParam interface {
	Call(*program.Caller, int64, int64, int64) (*types.Val, error)
}

type FourParam interface {
	Call(*program.Caller, int64, int64, int64, int64) (*types.Val, error)
}

type FiveParam interface {
	Call(*program.Caller, int64, int64, int64, int64, int64) (*types.Val, error)
}

type SixParam interface {
	Call(*program.Caller, int64, int64, int64, int64, int64, int64) (*types.Val, error)
}

// FnType is an interface for import functions with variadic int64 arguments
type FnType interface {
	Invoke(*program.Caller, ...int64) (*types.Val, error)
}

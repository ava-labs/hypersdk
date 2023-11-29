package host

import "github.com/ava-labs/hypersdk/x/programs/program"

type Import interface {
	// Name returns the name of this import module.
	Name() string
	// Register registers this import module with the provided link.
	Register(*Link) error
}

type OneParam interface {
	Call(*program.Caller, int64) (*program.Val, error)
}

type TwoParam interface {
	Call(*program.Caller, int64, int64) (*program.Val, error)
}

type ThreeParam interface {
	Call(*program.Caller, int64, int64, int64) (*program.Val, error)
}

type FourParam interface {
	Call(*program.Caller, int64, int64, int64, int64) (*program.Val, error)
}

type FiveParam interface {
	Call(*program.Caller, int64, int64, int64, int64, int64) (*program.Val, error)
}

type SixParam interface {
	Call(*program.Caller, int64, int64, int64, int64, int64, int64) (*program.Val, error)
}

// type FnTypes interface {
// 	OneParamFn | TwoParamFn | ThreeParamFn | FourParamFn | FiveParamFn | SixParamFn
// }

// FnType is an interface for import functions with variadic int64 arguments
type FnType interface {
	Invoke(*program.Caller, ...int64) (*program.Val, error)
}

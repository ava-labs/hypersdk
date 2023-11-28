package host

import (
	"github.com/ava-labs/hypersdk/x/programs/program"
)

type OneParam interface {
	Call(*program.Caller, int64) (*program.Val, error)
}

type OneParamFn func(*program.Caller, int64) (*program.Val, error)

func (fn OneParamFn) Call(c *program.Caller, arg1 int64) (*program.Val, error) {
	return fn(c, arg1)
}

type TwoParam interface {
	Call(*program.Caller, int64, int64) (*program.Val, error)
}

type TwoParamFn func(*program.Caller, int64, int64) (*program.Val, error)

func (fn TwoParamFn) Call(c *program.Caller, arg1, arg2 int64) (*program.Val, error) {
	return fn(c, arg1, arg2)
}

type ThreeParam interface {
	Call(*program.Caller, int64, int64, int64) (*program.Val, error)
}

type ThreeParamFn func(*program.Caller, int64, int64, int64) (*program.Val, error)

func (fn ThreeParamFn) Call(c *program.Caller, arg1, arg2, arg3 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3)
}

type FourParam interface {
	Call(*program.Caller, int64, int64, int64, int64) (*program.Val, error)
}

type FourParamFn func(*program.Caller, int64, int64, int64, int64) (*program.Val, error)

func (fn FourParamFn) Call(c *program.Caller, arg1, arg2, arg3, arg4 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3, arg4)
}

type FiveParam interface {
	Call(*program.Caller, int64, int64, int64, int64, int64) (*program.Val, error)
}

type FiveParamFn func(*program.Caller, int64, int64, int64, int64, int64) (*program.Val, error)

func (fn FiveParamFn) Call(c *program.Caller, arg1, arg2, arg3, arg4, arg5 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3, arg4, arg5)
}

type SixParam interface {
	Call(*program.Caller, int64, int64, int64, int64, int64, int64) (*program.Val, error)
}

type SixParamFn func(*program.Caller, int64, int64, int64, int64, int64, int64) (*program.Val, error)

func (fn SixParamFn) Call(c *program.Caller, arg1, arg2, arg3, arg4, arg5, arg6 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3, arg4, arg5, arg6)
}

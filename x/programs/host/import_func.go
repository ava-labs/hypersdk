package host

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/x/programs/program"
)

// ImportFn is a generic type that satisfies ImportFnType
type ImportFn[F FnTypes] struct {
	fn F
}

// Invoke calls the underlying function with the given arguments. Currently only
// supports int64 arguments and return values.
func (i ImportFn[F]) Invoke(c *program.Caller, args ...int64) (*program.Val, error) {
	switch fn := any(i.fn).(type) {
	case OneParam:
		return fn.Call(c, args[0])
	case TwoParam:
		return fn.Call(c, args[0], args[1])
	case ThreeParam:
		return fn.Call(c, args[0], args[1], args[2])
	case FourParam:
		return fn.Call(c, args[0], args[1], args[2], args[3])
	case FiveParam:
		return fn.Call(c, args[0], args[1], args[2], args[3], args[4])
	case SixParam:
		return fn.Call(c, args[0], args[1], args[2], args[3], args[4], args[5])
	default:
		return nil, fmt.Errorf("unsupported")
	}
}

// NewOneParamImport returns a new import function which accepts a single int64 parameter
func NewOneParamImport(name, module string, fn OneParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 1, newImportFnBuilder(fn).Build()
}

type OneParamFn func(*program.Caller, int64) (*program.Val, error)

func (fn OneParamFn) Call(c *program.Caller, arg1 int64) (*program.Val, error) {
	return fn(c, arg1)
}

// NewTwoParamImport returns a new import function which accepts two int64 parameters
func NewTwoParamImport(name, module string, fn TwoParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 2, newImportFnBuilder(fn).Build()
}

type TwoParamFn func(*program.Caller, int64, int64) (*program.Val, error)

func (fn TwoParamFn) Call(c *program.Caller, arg1, arg2 int64) (*program.Val, error) {
	return fn(c, arg1, arg2)
}

// NewThreeParamImport returns a new import function which accepts three int64 parameters
func NewThreeParamImport(name, module string, fn ThreeParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 3, newImportFnBuilder(fn).Build()
}

type ThreeParamFn func(*program.Caller, int64, int64, int64) (*program.Val, error)

func (fn ThreeParamFn) Call(c *program.Caller, arg1, arg2, arg3 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3)
}

// NewFourParamImport returns a new import function which accepts four int64 parameters
func NewFourParamImport(name, module string, fn FourParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 4, newImportFnBuilder(fn).Build()
}

type FourParamFn func(*program.Caller, int64, int64, int64, int64) (*program.Val, error)

func (fn FourParamFn) Call(c *program.Caller, arg1, arg2, arg3, arg4 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3, arg4)
}

// NewFiveParamImport returns a new import function which accepts five int64 parameters
func NewFiveParamImport(name, module string, fn FiveParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 5, newImportFnBuilder(fn).Build()
}

type FiveParamFn func(*program.Caller, int64, int64, int64, int64, int64) (*program.Val, error)

func (fn FiveParamFn) Call(c *program.Caller, arg1, arg2, arg3, arg4, arg5 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3, arg4, arg5)
}

// NewSixParamImport returns a new import function which accepts six int64 parameters
func NewSixParamImport(name, module string, fn SixParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 6, newImportFnBuilder(fn).Build()
}

type SixParamFn func(*program.Caller, int64, int64, int64, int64, int64, int64) (*program.Val, error)

func (fn SixParamFn) Call(c *program.Caller, arg1, arg2, arg3, arg4, arg5, arg6 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3, arg4, arg5, arg6)
}

func newImportFnBuilder[F FnTypes](fn F) *importFnBuilder[F] {
	return &importFnBuilder[F]{fn: fn}
}

// Generic Builder for ImportFn[T]
type importFnBuilder[F FnTypes] struct {
	fn F
}

func (b *importFnBuilder[F]) Build() func(caller *program.Caller, wargs ...wasmtime.Val) (*program.Val, error) {
	importFn := ImportFn[F]{fn: b.fn}
	fn := func(c *program.Caller, wargs ...wasmtime.Val) (*program.Val, error) {
		args := make([]int64, 0, len(wargs))
		for _, arg := range wargs {
			args = append(args, int64(arg.I64()))
		}
		return importFn.Invoke(c, args...)
	}
	return fn
}

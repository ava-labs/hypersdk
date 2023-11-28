package host

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/x/programs/program"
)

type FnTypes interface {
	OneParamFn | TwoParamFn | ThreeParamFn | FourParamFn | FiveParamFn | SixParamFn
}

// FnType is an interface for import functions with variadic int64 arguments
type FnType interface {
	Invoke(*program.Caller, ...int64) (*program.Val, error)
}

// Fn is a generic type that satisfies ImportFnType
type Fn[F FnTypes] struct {
	fn F
}

// Invoke calls the underlying function with the given arguments. Currently only
// supports int64 arguments and return values.
func (i Fn[F]) Invoke(c *program.Caller, args ...int64) (*program.Val, error) {
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

func NewOneParamImport(name, module string, fn OneParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 1, NewImportFnBuilder(fn).Build()
}

func NewTwoParamImport(name, module string, fn TwoParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 2, NewImportFnBuilder(fn).Build()
}

func NewThreeParamImport(name, module string, fn ThreeParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 3, NewImportFnBuilder(fn).Build()
}

func NewFourParamImport(name, module string, fn FourParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 4, NewImportFnBuilder(fn).Build()
}

func NewFiveParamImport(name, module string, fn FiveParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 5, NewImportFnBuilder(fn).Build()
}

func NewSixParamImport(name, module string, fn SixParamFn) (string, string, int, func(caller *program.Caller, args ...wasmtime.Val) (*program.Val, error)) {
	return name, module, 6, NewImportFnBuilder(fn).Build()
}

func NewImportFnBuilder[F FnTypes](fn F) *ImportFnBuilder[F] {
	return &ImportFnBuilder[F]{fn: fn}
}

// Generic Builder for ImportFn[T]
type ImportFnBuilder[F FnTypes] struct {
	fn F
}

func (b *ImportFnBuilder[F]) Build() func(caller *program.Caller, wargs ...wasmtime.Val) (*program.Val, error) {
	importFn := Fn[F]{fn: b.fn}
	fn := func(c *program.Caller, wargs ...wasmtime.Val) (*program.Val, error) {
		args := make([]int64, 0, len(wargs))
		for _, arg := range wargs {
			args = append(args, int64(arg.I64()))
		}
		return importFn.Invoke(c, args...)
	}
	return fn
}

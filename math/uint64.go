package math

import "github.com/ava-labs/avalanchego/utils/math"

type Uint64Operator struct {
	v   uint64
	err error
}

func NewUint64Operator(v uint64) *Uint64Operator {
	return &Uint64Operator{
		v: v,
	}
}

func (o *Uint64Operator) Add(n uint64) {
	if o.err != nil {
		return
	}

	nv, err := math.Add64(o.v, n)
	if err != nil {
		o.err = err
	}
	o.v = nv
}

func (o *Uint64Operator) Mul(n uint64) {
	if o.err != nil {
		return
	}

	nv, err := math.Mul64(o.v, n)
	if err != nil {
		o.err = err
	}
	o.v = nv
}

func (o *Uint64Operator) Value() (uint64, error) {
	return o.v, o.err
}

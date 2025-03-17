// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.13.3
// source: packer.go

package ws

import (
	"io"
	"reflect"
	"slices"
	"sync/atomic"
	"unicode/utf8"

	"github.com/StephenButtolph/canoto"
)

// Ensure that unused imports do not error
var (
	_ atomic.Int64

	_ = slices.Index[[]reflect.Type, reflect.Type]
	_ = io.ErrUnexpectedEOF
	_ = utf8.ValidString
)

const (
	canoto__txMessage__TxID__tag        = "\x0a" // canoto.Tag(1, canoto.Len)
	canoto__txMessage__ResultBytes__tag = "\x12" // canoto.Tag(2, canoto.Len)
)

type canotoData_txMessage struct {
	size atomic.Int64
}

// CanotoSpec returns the specification of this canoto message.
func (*txMessage) CanotoSpec(types ...reflect.Type) *canoto.Spec {
	types = append(types, reflect.TypeOf(txMessage{}))
	var zero txMessage
	s := &canoto.Spec{
		Name: "txMessage",
		Fields: []*canoto.FieldType{
			{
				FieldNumber:    1,
				Name:           "TxID",
				OneOf:          "",
				TypeFixedBytes: uint64(len(zero.TxID)),
			},
			{
				FieldNumber: 2,
				Name:        "ResultBytes",
				OneOf:       "",
				TypeBytes:   true,
			},
		},
	}
	s.CalculateCanotoCache()
	return s
}

// MakeCanoto creates a new empty value.
func (*txMessage) MakeCanoto() *txMessage {
	return new(txMessage)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// During parsing, the canoto cache is saved.
func (c *txMessage) UnmarshalCanoto(bytes []byte) error {
	r := canoto.Reader{
		B: bytes,
	}
	return c.UnmarshalCanotoFrom(r)
}

// UnmarshalCanotoFrom populates the struct from a canoto.Reader. Most users
// should just use UnmarshalCanoto.
//
// During parsing, the canoto cache is saved.
//
// This function enables configuration of reader options.
func (c *txMessage) UnmarshalCanotoFrom(r canoto.Reader) error {
	// Zero the struct before unmarshaling.
	*c = txMessage{}
	c.canotoData.size.Store(int64(len(r.B)))

	var minField uint32
	for canoto.HasNext(&r) {
		field, wireType, err := canoto.ReadTag(&r)
		if err != nil {
			return err
		}
		if field < minField {
			return canoto.ErrInvalidFieldOrder
		}

		switch field {
		case 1:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			const (
				expectedLength       = len(c.TxID)
				expectedLengthUint64 = uint64(expectedLength)
			)
			var length uint64
			if err := canoto.ReadUint(&r, &length); err != nil {
				return err
			}
			if length != expectedLengthUint64 {
				return canoto.ErrInvalidLength
			}
			if expectedLength > len(r.B) {
				return io.ErrUnexpectedEOF
			}

			copy((&c.TxID)[:], r.B)
			if canoto.IsZero(c.TxID) {
				return canoto.ErrZeroValue
			}
			r.B = r.B[expectedLength:]
		case 2:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadBytes(&r, &c.ResultBytes); err != nil {
				return err
			}
			if len(c.ResultBytes) == 0 {
				return canoto.ErrZeroValue
			}
		default:
			return canoto.ErrUnknownField
		}

		minField = field + 1
	}
	return nil
}

// ValidCanoto validates that the struct can be correctly marshaled into the
// Canoto format.
//
// Specifically, ValidCanoto ensures:
// 1. All OneOfs are specified at most once.
// 2. All strings are valid utf-8.
// 3. All custom fields are ValidCanoto.
func (c *txMessage) ValidCanoto() bool {
	if c == nil {
		return true
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
func (c *txMessage) CalculateCanotoCache() {
	if c == nil {
		return
	}
	var (
		size int
	)
	if !canoto.IsZero(c.TxID) {
		size += len(canoto__txMessage__TxID__tag) + canoto.SizeBytes((&c.TxID)[:])
	}
	if len(c.ResultBytes) != 0 {
		size += len(canoto__txMessage__ResultBytes__tag) + canoto.SizeBytes(c.ResultBytes)
	}
	c.canotoData.size.Store(int64(size))
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *txMessage) CachedCanotoSize() int {
	if c == nil {
		return 0
	}
	return int(c.canotoData.size.Load())
}

// MarshalCanoto returns the Canoto representation of this struct.
//
// It is assumed that this struct is ValidCanoto.
func (c *txMessage) MarshalCanoto() []byte {
	c.CalculateCanotoCache()
	w := canoto.Writer{
		B: make([]byte, 0, c.CachedCanotoSize()),
	}
	w = c.MarshalCanotoInto(w)
	return w.B
}

// MarshalCanotoInto writes the struct into a canoto.Writer and returns the
// resulting canoto.Writer. Most users should just use MarshalCanoto.
//
// It is assumed that CalculateCanotoCache has been called since the last
// modification to this struct.
//
// It is assumed that this struct is ValidCanoto.
func (c *txMessage) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	if !canoto.IsZero(c.TxID) {
		canoto.Append(&w, canoto__txMessage__TxID__tag)
		canoto.AppendBytes(&w, (&c.TxID)[:])
	}
	if len(c.ResultBytes) != 0 {
		canoto.Append(&w, canoto__txMessage__ResultBytes__tag)
		canoto.AppendBytes(&w, c.ResultBytes)
	}
	return w
}

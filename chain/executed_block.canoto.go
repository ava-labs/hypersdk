// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.13.3
// source: executed_block.go

package chain

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
	canoto__ExecutedBlock__Block__tag            = "\x0a" // canoto.Tag(1, canoto.Len)
	canoto__ExecutedBlock__ExecutionResults__tag = "\x12" // canoto.Tag(2, canoto.Len)
)

type canotoData_ExecutedBlock struct {
	size atomic.Int64
}

// CanotoSpec returns the specification of this canoto message.
func (*ExecutedBlock) CanotoSpec(types ...reflect.Type) *canoto.Spec {
	types = append(types, reflect.TypeOf(ExecutedBlock{}))
	var zero ExecutedBlock
	s := &canoto.Spec{
		Name: "ExecutedBlock",
		Fields: []*canoto.FieldType{
			canoto.FieldTypeFromField(
				/*type inference:*/ (zero.Block),
				/*FieldNumber:   */ 1,
				/*Name:          */ "Block",
				/*FixedLength:   */ 0,
				/*Repeated:      */ false,
				/*OneOf:         */ "",
				/*types:         */ types,
			),
			canoto.FieldTypeFromField(
				/*type inference:*/ (zero.ExecutionResults),
				/*FieldNumber:   */ 2,
				/*Name:          */ "ExecutionResults",
				/*FixedLength:   */ 0,
				/*Repeated:      */ false,
				/*OneOf:         */ "",
				/*types:         */ types,
			),
		},
	}
	s.CalculateCanotoCache()
	return s
}

// MakeCanoto creates a new empty value.
func (*ExecutedBlock) MakeCanoto() *ExecutedBlock {
	return new(ExecutedBlock)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// During parsing, the canoto cache is saved.
func (c *ExecutedBlock) UnmarshalCanoto(bytes []byte) error {
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
func (c *ExecutedBlock) UnmarshalCanotoFrom(r canoto.Reader) error {
	// Zero the struct before unmarshaling.
	*c = ExecutedBlock{}
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

			// Read the bytes for the field.
			originalUnsafe := r.Unsafe
			r.Unsafe = true
			var msgBytes []byte
			if err := canoto.ReadBytes(&r, &msgBytes); err != nil {
				return err
			}
			if len(msgBytes) == 0 {
				return canoto.ErrZeroValue
			}
			r.Unsafe = originalUnsafe

			// Unmarshal the field from the bytes.
			remainingBytes := r.B
			r.B = msgBytes
			c.Block = canoto.MakePointer(c.Block)
			if err := (c.Block).UnmarshalCanotoFrom(r); err != nil {
				return err
			}
			r.B = remainingBytes
		case 2:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			// Read the bytes for the field.
			originalUnsafe := r.Unsafe
			r.Unsafe = true
			var msgBytes []byte
			if err := canoto.ReadBytes(&r, &msgBytes); err != nil {
				return err
			}
			if len(msgBytes) == 0 {
				return canoto.ErrZeroValue
			}
			r.Unsafe = originalUnsafe

			// Unmarshal the field from the bytes.
			remainingBytes := r.B
			r.B = msgBytes
			c.ExecutionResults = canoto.MakePointer(c.ExecutionResults)
			if err := (c.ExecutionResults).UnmarshalCanotoFrom(r); err != nil {
				return err
			}
			r.B = remainingBytes
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
func (c *ExecutedBlock) ValidCanoto() bool {
	if c == nil {
		return true
	}
	if c.Block != nil && !(c.Block).ValidCanoto() {
		return false
	}
	if c.ExecutionResults != nil && !(c.ExecutionResults).ValidCanoto() {
		return false
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
func (c *ExecutedBlock) CalculateCanotoCache() {
	if c == nil {
		return
	}
	var (
		size int
	)
	if c.Block != nil {
		(c.Block).CalculateCanotoCache()
		if fieldSize := (c.Block).CachedCanotoSize(); fieldSize != 0 {
			size += len(canoto__ExecutedBlock__Block__tag) + canoto.SizeUint(uint64(fieldSize)) + fieldSize
		}
	}
	if c.ExecutionResults != nil {
		(c.ExecutionResults).CalculateCanotoCache()
		if fieldSize := (c.ExecutionResults).CachedCanotoSize(); fieldSize != 0 {
			size += len(canoto__ExecutedBlock__ExecutionResults__tag) + canoto.SizeUint(uint64(fieldSize)) + fieldSize
		}
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
func (c *ExecutedBlock) CachedCanotoSize() int {
	if c == nil {
		return 0
	}
	return int(c.canotoData.size.Load())
}

// MarshalCanoto returns the Canoto representation of this struct.
//
// It is assumed that this struct is ValidCanoto.
func (c *ExecutedBlock) MarshalCanoto() []byte {
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
func (c *ExecutedBlock) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	if c.Block != nil {
		if fieldSize := (c.Block).CachedCanotoSize(); fieldSize != 0 {
			canoto.Append(&w, canoto__ExecutedBlock__Block__tag)
			canoto.AppendUint(&w, uint64(fieldSize))
			w = (c.Block).MarshalCanotoInto(w)
		}
	}
	if c.ExecutionResults != nil {
		if fieldSize := (c.ExecutionResults).CachedCanotoSize(); fieldSize != 0 {
			canoto.Append(&w, canoto__ExecutedBlock__ExecutionResults__tag)
			canoto.AppendUint(&w, uint64(fieldSize))
			w = (c.ExecutionResults).MarshalCanotoInto(w)
		}
	}
	return w
}

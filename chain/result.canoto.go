// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.10.0
// source: chain/result.go

package chain

import (
	"io"
	"sync/atomic"
	"unicode/utf8"

	"github.com/StephenButtolph/canoto"
)

// Ensure that unused imports do not error
var (
	_ atomic.Int64

	_ = io.ErrUnexpectedEOF
	_ = utf8.ValidString
)

const (
	canoto__Result__Success__tag = "\x08" // canoto.Tag(1, canoto.Varint)
	canoto__Result__Error__tag   = "\x12" // canoto.Tag(2, canoto.Len)
	canoto__Result__Outputs__tag = "\x1a" // canoto.Tag(3, canoto.Len)
	canoto__Result__Units__tag   = "\x22" // canoto.Tag(4, canoto.Len)
	canoto__Result__Fee__tag     = "\x29" // canoto.Tag(5, canoto.I64)
)

type canotoData_Result struct {
	// Enforce noCopy before atomic usage.
	// See https://github.com/StephenButtolph/canoto/pull/32
	_ atomic.Int64

	size      int
	UnitsSize int
}

// MakeCanoto creates a new empty value.
func (*Result) MakeCanoto() *Result {
	return new(Result)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *Result) UnmarshalCanoto(bytes []byte) error {
	r := canoto.Reader{
		B: bytes,
	}
	return c.UnmarshalCanotoFrom(r)
}

// UnmarshalCanotoFrom populates the struct from a canoto.Reader. Most users
// should just use UnmarshalCanoto.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
//
// This function enables configuration of reader options.
func (c *Result) UnmarshalCanotoFrom(r canoto.Reader) error {
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
			if wireType != canoto.Varint {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadBool(&r, &c.Success); err != nil {
				return err
			}
			if canoto.IsZero(c.Success) {
				return canoto.ErrZeroValue
			}
		case 2:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadBytes(&r, &c.Error); err != nil {
				return err
			}
			if len(c.Error) == 0 {
				return canoto.ErrZeroValue
			}
		case 3:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			remainingBytes := r.B
			originalUnsafe := r.Unsafe
			r.Unsafe = true
			err := canoto.ReadBytes(&r, new([]byte))
			r.Unsafe = originalUnsafe
			if err != nil {
				return err
			}

			count, err := canoto.CountBytes(r.B, canoto__Result__Outputs__tag)
			if err != nil {
				return err
			}
			c.Outputs = canoto.MakeSlice(c.Outputs, 1+count)

			r.B = remainingBytes
			if err := canoto.ReadBytes(&r, &c.Outputs[0]); err != nil {
				return err
			}
			for i := range count {
				r.B = r.B[len(canoto__Result__Outputs__tag):]
				if err := canoto.ReadBytes(&r, &c.Outputs[1+i]); err != nil {
					return err
				}
			}
		case 4:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			originalUnsafe := r.Unsafe
			r.Unsafe = true
			var msgBytes []byte
			err := canoto.ReadBytes(&r, &msgBytes)
			r.Unsafe = originalUnsafe
			if err != nil {
				return err
			}

			remainingBytes := r.B
			r.B = msgBytes
			for i := range c.Units {
				if err := canoto.ReadInt(&r, &c.Units[i]); err != nil {
					r.B = remainingBytes
					return err
				}
			}
			hasNext := canoto.HasNext(&r)
			r.B = remainingBytes
			if hasNext {
				return io.ErrUnexpectedEOF
			}
			if canoto.IsZero(c.Units) {
				return canoto.ErrZeroValue
			}
		case 5:
			if wireType != canoto.I64 {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadFint64(&r, &c.Fee); err != nil {
				return err
			}
			if canoto.IsZero(c.Fee) {
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
func (c *Result) ValidCanoto() bool {
	if c == nil {
		return true
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *Result) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	if !canoto.IsZero(c.Success) {
		c.canotoData.size += len(canoto__Result__Success__tag) + canoto.SizeBool
	}
	if len(c.Error) != 0 {
		c.canotoData.size += len(canoto__Result__Error__tag) + canoto.SizeBytes(c.Error)
	}
	for _, v := range c.Outputs {
		c.canotoData.size += len(canoto__Result__Outputs__tag) + canoto.SizeBytes(v)
	}
	if !canoto.IsZero(c.Units) {
		c.canotoData.UnitsSize = 0
		for _, v := range c.Units {
			c.canotoData.UnitsSize += canoto.SizeInt(v)
		}
		c.canotoData.size += len(canoto__Result__Units__tag) + canoto.SizeInt(int64(c.canotoData.UnitsSize)) + c.canotoData.UnitsSize
	}
	if !canoto.IsZero(c.Fee) {
		c.canotoData.size += len(canoto__Result__Fee__tag) + canoto.SizeFint64
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *Result) CachedCanotoSize() int {
	if c == nil {
		return 0
	}
	return c.canotoData.size
}

// MarshalCanoto returns the Canoto representation of this struct.
//
// It is assumed that this struct is ValidCanoto.
//
// It is not safe to call this function concurrently.
func (c *Result) MarshalCanoto() []byte {
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
//
// It is not safe to call this function concurrently.
func (c *Result) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	if !canoto.IsZero(c.Success) {
		canoto.Append(&w, canoto__Result__Success__tag)
		canoto.AppendBool(&w, true)
	}
	if len(c.Error) != 0 {
		canoto.Append(&w, canoto__Result__Error__tag)
		canoto.AppendBytes(&w, c.Error)
	}
	for _, v := range c.Outputs {
		canoto.Append(&w, canoto__Result__Outputs__tag)
		canoto.AppendBytes(&w, v)
	}
	if !canoto.IsZero(c.Units) {
		canoto.Append(&w, canoto__Result__Units__tag)
		canoto.AppendInt(&w, int64(c.canotoData.UnitsSize))
		for _, v := range c.Units {
			canoto.AppendInt(&w, v)
		}
	}
	if !canoto.IsZero(c.Fee) {
		canoto.Append(&w, canoto__Result__Fee__tag)
		canoto.AppendFint64(&w, c.Fee)
	}
	return w
}

const (
	canoto__ExecutionResults__Results__tag       = "\x0a" // canoto.Tag(1, canoto.Len)
	canoto__ExecutionResults__UnitPrices__tag    = "\x12" // canoto.Tag(2, canoto.Len)
	canoto__ExecutionResults__UnitsConsumed__tag = "\x1a" // canoto.Tag(3, canoto.Len)
)

type canotoData_ExecutionResults struct {
	// Enforce noCopy before atomic usage.
	// See https://github.com/StephenButtolph/canoto/pull/32
	_ atomic.Int64

	size int
}

// MakeCanoto creates a new empty value.
func (*ExecutionResults) MakeCanoto() *ExecutionResults {
	return new(ExecutionResults)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *ExecutionResults) UnmarshalCanoto(bytes []byte) error {
	r := canoto.Reader{
		B: bytes,
	}
	return c.UnmarshalCanotoFrom(r)
}

// UnmarshalCanotoFrom populates the struct from a canoto.Reader. Most users
// should just use UnmarshalCanoto.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
//
// This function enables configuration of reader options.
func (c *ExecutionResults) UnmarshalCanotoFrom(r canoto.Reader) error {
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

			originalUnsafe := r.Unsafe
			r.Unsafe = true
			var msgBytes []byte
			err := canoto.ReadBytes(&r, &msgBytes)
			r.Unsafe = originalUnsafe
			if err != nil {
				return err
			}

			remainingBytes := r.B
			count, err := canoto.CountBytes(remainingBytes, canoto__ExecutionResults__Results__tag)
			if err != nil {
				return err
			}

			c.Results = canoto.MakeSlice(c.Results, 1+count)
			if len(msgBytes) != 0 {
				r.B = msgBytes
				c.Results[0] = c.Results[0].MakeCanoto()
				err = c.Results[0].UnmarshalCanotoFrom(r)
				r.B = remainingBytes
				if err != nil {
					return err
				}
			}

			for i := range count {
				r.B = r.B[len(canoto__ExecutionResults__Results__tag):]
				r.Unsafe = true
				err := canoto.ReadBytes(&r, &msgBytes)
				r.Unsafe = originalUnsafe
				if err != nil {
					return err
				}
				if len(msgBytes) == 0 {
					continue
				}

				remainingBytes := r.B
				r.B = msgBytes
				c.Results[1+i] = c.Results[1+i].MakeCanoto()
				err = c.Results[1+i].UnmarshalCanotoFrom(r)
				r.B = remainingBytes
				if err != nil {
					return err
				}
			}
		case 2:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			originalUnsafe := r.Unsafe
			r.Unsafe = true
			var msgBytes []byte
			err := canoto.ReadBytes(&r, &msgBytes)
			r.Unsafe = originalUnsafe
			if err != nil {
				return err
			}

			remainingBytes := r.B
			r.B = msgBytes
			for i := range c.UnitPrices {
				if err := canoto.ReadFint64(&r, &c.UnitPrices[i]); err != nil {
					r.B = remainingBytes
					return err
				}
			}
			hasNext := canoto.HasNext(&r)
			r.B = remainingBytes
			if hasNext {
				return io.ErrUnexpectedEOF
			}
			if canoto.IsZero(c.UnitPrices) {
				return canoto.ErrZeroValue
			}
		case 3:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			originalUnsafe := r.Unsafe
			r.Unsafe = true
			var msgBytes []byte
			err := canoto.ReadBytes(&r, &msgBytes)
			r.Unsafe = originalUnsafe
			if err != nil {
				return err
			}

			remainingBytes := r.B
			r.B = msgBytes
			for i := range c.UnitsConsumed {
				if err := canoto.ReadFint64(&r, &c.UnitsConsumed[i]); err != nil {
					r.B = remainingBytes
					return err
				}
			}
			hasNext := canoto.HasNext(&r)
			r.B = remainingBytes
			if hasNext {
				return io.ErrUnexpectedEOF
			}
			if canoto.IsZero(c.UnitsConsumed) {
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
func (c *ExecutionResults) ValidCanoto() bool {
	if c == nil {
		return true
	}
	for i := range c.Results {
		if !c.Results[i].ValidCanoto() {
			return false
		}
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *ExecutionResults) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	for i := range c.Results {
		c.Results[i].CalculateCanotoCache()
		fieldSize := c.Results[i].CachedCanotoSize()
		c.canotoData.size += len(canoto__ExecutionResults__Results__tag) + canoto.SizeInt(int64(fieldSize)) + fieldSize
	}
	if !canoto.IsZero(c.UnitPrices) {
		const fieldSize = len(c.UnitPrices) * canoto.SizeFint64
		c.canotoData.size += len(canoto__ExecutionResults__UnitPrices__tag) + fieldSize + canoto.SizeInt(int64(fieldSize))
	}
	if !canoto.IsZero(c.UnitsConsumed) {
		const fieldSize = len(c.UnitsConsumed) * canoto.SizeFint64
		c.canotoData.size += len(canoto__ExecutionResults__UnitsConsumed__tag) + fieldSize + canoto.SizeInt(int64(fieldSize))
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *ExecutionResults) CachedCanotoSize() int {
	if c == nil {
		return 0
	}
	return c.canotoData.size
}

// MarshalCanoto returns the Canoto representation of this struct.
//
// It is assumed that this struct is ValidCanoto.
//
// It is not safe to call this function concurrently.
func (c *ExecutionResults) MarshalCanoto() []byte {
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
//
// It is not safe to call this function concurrently.
func (c *ExecutionResults) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	for i := range c.Results {
		canoto.Append(&w, canoto__ExecutionResults__Results__tag)
		fieldSize := c.Results[i].CachedCanotoSize()
		canoto.AppendInt(&w, int64(fieldSize))
		if fieldSize != 0 {
			w = c.Results[i].MarshalCanotoInto(w)
		}
	}
	if !canoto.IsZero(c.UnitPrices) {
		const fieldSize = len(c.UnitPrices) * canoto.SizeFint64
		canoto.Append(&w, canoto__ExecutionResults__UnitPrices__tag)
		canoto.AppendInt(&w, int64(fieldSize))
		for _, v := range c.UnitPrices {
			canoto.AppendFint64(&w, v)
		}
	}
	if !canoto.IsZero(c.UnitsConsumed) {
		const fieldSize = len(c.UnitsConsumed) * canoto.SizeFint64
		canoto.Append(&w, canoto__ExecutionResults__UnitsConsumed__tag)
		canoto.AppendInt(&w, int64(fieldSize))
		for _, v := range c.UnitsConsumed {
			canoto.AppendFint64(&w, v)
		}
	}
	return w
}

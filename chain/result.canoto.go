// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.13.3
// source: chain/result.go

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
	canoto__Result__Success__tag = "\x08" // canoto.Tag(1, canoto.Varint)
	canoto__Result__Error__tag   = "\x12" // canoto.Tag(2, canoto.Len)
	canoto__Result__Outputs__tag = "\x1a" // canoto.Tag(3, canoto.Len)
	canoto__Result__Units__tag   = "\x22" // canoto.Tag(4, canoto.Len)
	canoto__Result__Fee__tag     = "\x29" // canoto.Tag(5, canoto.I64)
)

type canotoData_Result struct {
	size      atomic.Int64
	UnitsSize atomic.Int64
}

// CanotoSpec returns the specification of this canoto message.
func (*Result) CanotoSpec(types ...reflect.Type) *canoto.Spec {
	types = append(types, reflect.TypeOf(Result{}))
	var zero Result
	s := &canoto.Spec{
		Name: "Result",
		Fields: []*canoto.FieldType{
			{
				FieldNumber: 1,
				Name:        "Success",
				OneOf:       "",
				TypeBool:    true,
			},
			{
				FieldNumber: 2,
				Name:        "Error",
				OneOf:       "",
				TypeBytes:   true,
			},
			{
				FieldNumber: 3,
				Name:        "Outputs",
				Repeated:    true,
				OneOf:       "",
				TypeBytes:   true,
			},
			{
				FieldNumber: 4,
				Name:        "Units",
				FixedLength: uint64(len(zero.Units)),
				Repeated:    true,
				OneOf:       "",
				TypeUint:    canoto.SizeOf(canoto.MakeEntry(zero.Units[:])),
			},
			canoto.FieldTypeFromFint(
				/*type inference:*/ zero.Fee,
				/*FieldNumber:   */ 5,
				/*Name:          */ "Fee",
				/*FixedLength:   */ 0,
				/*Repeated:      */ false,
				/*OneOf:         */ "",
			),
		},
	}
	s.CalculateCanotoCache()
	return s
}

// MakeCanoto creates a new empty value.
func (*Result) MakeCanoto() *Result {
	return new(Result)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// During parsing, the canoto cache is saved.
func (c *Result) UnmarshalCanoto(bytes []byte) error {
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
func (c *Result) UnmarshalCanotoFrom(r canoto.Reader) error {
	// Zero the struct before unmarshaling.
	*c = Result{}
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

			// Skip the first entry because we have already stripped the tag.
			remainingBytes := r.B
			originalUnsafe := r.Unsafe
			r.Unsafe = true
			if err := canoto.ReadBytes(&r, new([]byte)); err != nil {
				return err
			}
			r.Unsafe = originalUnsafe

			// Count the number of additional entries after the first entry.
			countMinus1, err := canoto.CountBytes(r.B, canoto__Result__Outputs__tag)
			if err != nil {
				return err
			}
			c.Outputs = canoto.MakeSlice(c.Outputs, countMinus1+1)

			// Read the first entry manually because the tag is still already
			// stripped.
			r.B = remainingBytes
			if err := canoto.ReadBytes(&r, &c.Outputs[0]); err != nil {
				return err
			}

			// Read the rest of the entries, stripping the tag each time.
			for i := range countMinus1 {
				r.B = r.B[len(canoto__Result__Outputs__tag):]
				if err := canoto.ReadBytes(&r, &c.Outputs[1+i]); err != nil {
					return err
				}
			}
		case 4:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			// Read the packed field bytes.
			originalUnsafe := r.Unsafe
			r.Unsafe = true
			var msgBytes []byte
			if err := canoto.ReadBytes(&r, &msgBytes); err != nil {
				return err
			}
			r.Unsafe = originalUnsafe

			// Read each value from the packed field bytes into the array.
			remainingBytes := r.B
			r.B = msgBytes
			for i := range &c.Units {
				if err := canoto.ReadUint(&r, &(&c.Units)[i]); err != nil {
					return err
				}
			}
			if canoto.HasNext(&r) {
				return canoto.ErrInvalidLength
			}
			if canoto.IsZero(c.Units) {
				return canoto.ErrZeroValue
			}
			r.B = remainingBytes
			c.canotoData.UnitsSize.Store(int64(len(msgBytes)))
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
func (c *Result) CalculateCanotoCache() {
	if c == nil {
		return
	}
	var (
		size int
	)
	if !canoto.IsZero(c.Success) {
		size += len(canoto__Result__Success__tag) + canoto.SizeBool
	}
	if len(c.Error) != 0 {
		size += len(canoto__Result__Error__tag) + canoto.SizeBytes(c.Error)
	}
	for _, v := range c.Outputs {
		size += len(canoto__Result__Outputs__tag) + canoto.SizeBytes(v)
	}
	if !canoto.IsZero(c.Units) {
		var fieldSize int
		for _, v := range &c.Units {
			fieldSize += canoto.SizeUint(v)
		}
		size += len(canoto__Result__Units__tag) + canoto.SizeUint(uint64(fieldSize)) + fieldSize
		c.canotoData.UnitsSize.Store(int64(fieldSize))
	}
	if !canoto.IsZero(c.Fee) {
		size += len(canoto__Result__Fee__tag) + canoto.SizeFint64
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
func (c *Result) CachedCanotoSize() int {
	if c == nil {
		return 0
	}
	return int(c.canotoData.size.Load())
}

// MarshalCanoto returns the Canoto representation of this struct.
//
// It is assumed that this struct is ValidCanoto.
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
		canoto.AppendUint(&w, uint64(c.canotoData.UnitsSize.Load()))
		for _, v := range &c.Units {
			canoto.AppendUint(&w, v)
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
	size atomic.Int64
}

// CanotoSpec returns the specification of this canoto message.
func (*ExecutionResults) CanotoSpec(types ...reflect.Type) *canoto.Spec {
	types = append(types, reflect.TypeOf(ExecutionResults{}))
	var zero ExecutionResults
	s := &canoto.Spec{
		Name: "ExecutionResults",
		Fields: []*canoto.FieldType{
			canoto.FieldTypeFromField(
				/*type inference:*/ canoto.MakeEntry(zero.Results),
				/*FieldNumber:   */ 1,
				/*Name:          */ "Results",
				/*FixedLength:   */ 0,
				/*Repeated:      */ true,
				/*OneOf:         */ "",
				/*types:         */ types,
			),
			canoto.FieldTypeFromFint(
				/*type inference:*/ canoto.MakeEntry(zero.UnitPrices[:]),
				/*FieldNumber:   */ 2,
				/*Name:          */ "UnitPrices",
				/*FixedLength:   */ uint64(len(zero.UnitPrices)),
				/*Repeated:      */ true,
				/*OneOf:         */ "",
			),
			canoto.FieldTypeFromFint(
				/*type inference:*/ canoto.MakeEntry(zero.UnitsConsumed[:]),
				/*FieldNumber:   */ 3,
				/*Name:          */ "UnitsConsumed",
				/*FixedLength:   */ uint64(len(zero.UnitsConsumed)),
				/*Repeated:      */ true,
				/*OneOf:         */ "",
			),
		},
	}
	s.CalculateCanotoCache()
	return s
}

// MakeCanoto creates a new empty value.
func (*ExecutionResults) MakeCanoto() *ExecutionResults {
	return new(ExecutionResults)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// During parsing, the canoto cache is saved.
func (c *ExecutionResults) UnmarshalCanoto(bytes []byte) error {
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
func (c *ExecutionResults) UnmarshalCanotoFrom(r canoto.Reader) error {
	// Zero the struct before unmarshaling.
	*c = ExecutionResults{}
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

			// Read the first entry manually because the tag is already
			// stripped.
			originalUnsafe := r.Unsafe
			r.Unsafe = true
			var msgBytes []byte
			if err := canoto.ReadBytes(&r, &msgBytes); err != nil {
				return err
			}
			r.Unsafe = originalUnsafe

			// Count the number of additional entries after the first entry.
			countMinus1, err := canoto.CountBytes(r.B, canoto__ExecutionResults__Results__tag)
			if err != nil {
				return err
			}

			c.Results = canoto.MakeSlice(c.Results, countMinus1+1)
			if len(msgBytes) != 0 {
				remainingBytes := r.B
				r.B = msgBytes
				c.Results[0] = c.Results[0].MakeCanoto()
				if err := c.Results[0].UnmarshalCanotoFrom(r); err != nil {
					return err
				}
				r.B = remainingBytes
			}

			// Read the rest of the entries, stripping the tag each time.
			for i := range countMinus1 {
				r.B = r.B[len(canoto__ExecutionResults__Results__tag):]
				r.Unsafe = true
				if err := canoto.ReadBytes(&r, &msgBytes); err != nil {
					return err
				}
				if len(msgBytes) == 0 {
					continue
				}
				r.Unsafe = originalUnsafe

				remainingBytes := r.B
				r.B = msgBytes
				c.Results[1+i] = c.Results[1+i].MakeCanoto()
				if err := c.Results[1+i].UnmarshalCanotoFrom(r); err != nil {
					return err
				}
				r.B = remainingBytes
			}
		case 2:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			// Read the packed field bytes.
			originalUnsafe := r.Unsafe
			r.Unsafe = true
			var msgBytes []byte
			if err := canoto.ReadBytes(&r, &msgBytes); err != nil {
				return err
			}
			r.Unsafe = originalUnsafe

			// Read each value from the packed field bytes into the array.
			remainingBytes := r.B
			r.B = msgBytes
			for i := range &c.UnitPrices {
				if err := canoto.ReadFint64(&r, &(&c.UnitPrices)[i]); err != nil {
					return err
				}
			}
			if canoto.HasNext(&r) {
				return canoto.ErrInvalidLength
			}
			if canoto.IsZero(c.UnitPrices) {
				return canoto.ErrZeroValue
			}
			r.B = remainingBytes
		case 3:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			// Read the packed field bytes.
			originalUnsafe := r.Unsafe
			r.Unsafe = true
			var msgBytes []byte
			if err := canoto.ReadBytes(&r, &msgBytes); err != nil {
				return err
			}
			r.Unsafe = originalUnsafe

			// Read each value from the packed field bytes into the array.
			remainingBytes := r.B
			r.B = msgBytes
			for i := range &c.UnitsConsumed {
				if err := canoto.ReadFint64(&r, &(&c.UnitsConsumed)[i]); err != nil {
					return err
				}
			}
			if canoto.HasNext(&r) {
				return canoto.ErrInvalidLength
			}
			if canoto.IsZero(c.UnitsConsumed) {
				return canoto.ErrZeroValue
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
func (c *ExecutionResults) CalculateCanotoCache() {
	if c == nil {
		return
	}
	var (
		size int
	)
	for i := range c.Results {
		c.Results[i].CalculateCanotoCache()
		fieldSize := c.Results[i].CachedCanotoSize()
		size += len(canoto__ExecutionResults__Results__tag) + canoto.SizeUint(uint64(fieldSize)) + fieldSize
	}
	if !canoto.IsZero(c.UnitPrices) {
		const fieldSize = len(c.UnitPrices) * canoto.SizeFint64
		size += len(canoto__ExecutionResults__UnitPrices__tag) + fieldSize + canoto.SizeUint(uint64(fieldSize))
	}
	if !canoto.IsZero(c.UnitsConsumed) {
		const fieldSize = len(c.UnitsConsumed) * canoto.SizeFint64
		size += len(canoto__ExecutionResults__UnitsConsumed__tag) + fieldSize + canoto.SizeUint(uint64(fieldSize))
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
func (c *ExecutionResults) CachedCanotoSize() int {
	if c == nil {
		return 0
	}
	return int(c.canotoData.size.Load())
}

// MarshalCanoto returns the Canoto representation of this struct.
//
// It is assumed that this struct is ValidCanoto.
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
func (c *ExecutionResults) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	for i := range c.Results {
		canoto.Append(&w, canoto__ExecutionResults__Results__tag)
		fieldSize := c.Results[i].CachedCanotoSize()
		canoto.AppendUint(&w, uint64(fieldSize))
		if fieldSize != 0 {
			w = c.Results[i].MarshalCanotoInto(w)
		}
	}
	if !canoto.IsZero(c.UnitPrices) {
		const fieldSize = len(c.UnitPrices) * canoto.SizeFint64
		canoto.Append(&w, canoto__ExecutionResults__UnitPrices__tag)
		canoto.AppendUint(&w, uint64(fieldSize))
		for _, v := range &c.UnitPrices {
			canoto.AppendFint64(&w, v)
		}
	}
	if !canoto.IsZero(c.UnitsConsumed) {
		const fieldSize = len(c.UnitsConsumed) * canoto.SizeFint64
		canoto.Append(&w, canoto__ExecutionResults__UnitsConsumed__tag)
		canoto.AppendUint(&w, uint64(fieldSize))
		for _, v := range &c.UnitsConsumed {
			canoto.AppendFint64(&w, v)
		}
	}
	return w
}

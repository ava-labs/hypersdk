// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.13.3
// source: transaction_codec.go

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
	canoto__SerializeTx__Base__tag    = "\x0a" // canoto.Tag(1, canoto.Len)
	canoto__SerializeTx__Actions__tag = "\x12" // canoto.Tag(2, canoto.Len)
	canoto__SerializeTx__Auth__tag    = "\x1a" // canoto.Tag(3, canoto.Len)
)

type canotoData_SerializeTx struct {
	size atomic.Int64
}

// CanotoSpec returns the specification of this canoto message.
func (*SerializeTx) CanotoSpec(types ...reflect.Type) *canoto.Spec {
	types = append(types, reflect.TypeOf(SerializeTx{}))
	var zero SerializeTx
	s := &canoto.Spec{
		Name: "SerializeTx",
		Fields: []*canoto.FieldType{
			canoto.FieldTypeFromField(
				/*type inference:*/ (&zero.Base),
				/*FieldNumber:   */ 1,
				/*Name:          */ "Base",
				/*FixedLength:   */ 0,
				/*Repeated:      */ false,
				/*OneOf:         */ "",
				/*types:         */ types,
			),
			{
				FieldNumber: 2,
				Name:        "Actions",
				Repeated:    true,
				OneOf:       "",
				TypeBytes:   true,
			},
			{
				FieldNumber: 3,
				Name:        "Auth",
				OneOf:       "",
				TypeBytes:   true,
			},
		},
	}
	s.CalculateCanotoCache()
	return s
}

// MakeCanoto creates a new empty value.
func (*SerializeTx) MakeCanoto() *SerializeTx {
	return new(SerializeTx)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// During parsing, the canoto cache is saved.
func (c *SerializeTx) UnmarshalCanoto(bytes []byte) error {
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
func (c *SerializeTx) UnmarshalCanotoFrom(r canoto.Reader) error {
	// Zero the struct before unmarshaling.
	*c = SerializeTx{}
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
			if err := (&c.Base).UnmarshalCanotoFrom(r); err != nil {
				return err
			}
			r.B = remainingBytes
		case 2:
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
			countMinus1, err := canoto.CountBytes(r.B, canoto__SerializeTx__Actions__tag)
			if err != nil {
				return err
			}
			c.Actions = canoto.MakeSlice(c.Actions, countMinus1+1)

			// Read the first entry manually because the tag is still already
			// stripped.
			r.B = remainingBytes
			if err := canoto.ReadBytes(&r, &c.Actions[0]); err != nil {
				return err
			}

			// Read the rest of the entries, stripping the tag each time.
			for i := range countMinus1 {
				r.B = r.B[len(canoto__SerializeTx__Actions__tag):]
				if err := canoto.ReadBytes(&r, &c.Actions[1+i]); err != nil {
					return err
				}
			}
		case 3:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadBytes(&r, &c.Auth); err != nil {
				return err
			}
			if len(c.Auth) == 0 {
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
func (c *SerializeTx) ValidCanoto() bool {
	if c == nil {
		return true
	}
	if !(&c.Base).ValidCanoto() {
		return false
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
func (c *SerializeTx) CalculateCanotoCache() {
	if c == nil {
		return
	}
	var (
		size int
	)
	(&c.Base).CalculateCanotoCache()
	if fieldSize := (&c.Base).CachedCanotoSize(); fieldSize != 0 {
		size += len(canoto__SerializeTx__Base__tag) + canoto.SizeUint(uint64(fieldSize)) + fieldSize
	}
	for _, v := range c.Actions {
		size += len(canoto__SerializeTx__Actions__tag) + canoto.SizeBytes(v)
	}
	if len(c.Auth) != 0 {
		size += len(canoto__SerializeTx__Auth__tag) + canoto.SizeBytes(c.Auth)
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
func (c *SerializeTx) CachedCanotoSize() int {
	if c == nil {
		return 0
	}
	return int(c.canotoData.size.Load())
}

// MarshalCanoto returns the Canoto representation of this struct.
//
// It is assumed that this struct is ValidCanoto.
func (c *SerializeTx) MarshalCanoto() []byte {
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
func (c *SerializeTx) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	if fieldSize := (&c.Base).CachedCanotoSize(); fieldSize != 0 {
		canoto.Append(&w, canoto__SerializeTx__Base__tag)
		canoto.AppendUint(&w, uint64(fieldSize))
		w = (&c.Base).MarshalCanotoInto(w)
	}
	for _, v := range c.Actions {
		canoto.Append(&w, canoto__SerializeTx__Actions__tag)
		canoto.AppendBytes(&w, v)
	}
	if len(c.Auth) != 0 {
		canoto.Append(&w, canoto__SerializeTx__Auth__tag)
		canoto.AppendBytes(&w, c.Auth)
	}
	return w
}

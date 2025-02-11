// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.10.0
// source: auth/secp256r1.go

package auth

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
	canoto__SECP256R1__Signer__tag    = "\x0a" // canoto.Tag(1, canoto.Len)
	canoto__SECP256R1__Signature__tag = "\x12" // canoto.Tag(2, canoto.Len)
)

type canotoData_SECP256R1 struct {
	// Enforce noCopy before atomic usage.
	// See https://github.com/StephenButtolph/canoto/pull/32
	_ atomic.Int64

	size int
}

// MakeCanoto creates a new empty value.
func (*SECP256R1) MakeCanoto() *SECP256R1 {
	return new(SECP256R1)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *SECP256R1) UnmarshalCanoto(bytes []byte) error {
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
func (c *SECP256R1) UnmarshalCanotoFrom(r canoto.Reader) error {
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

			var length int64
			if err := canoto.ReadInt(&r, &length); err != nil {
				return err
			}

			const (
				expectedLength      = len(c.Signer)
				expectedLengthInt64 = int64(expectedLength)
			)
			if length != expectedLengthInt64 {
				return canoto.ErrInvalidLength
			}
			if expectedLength > len(r.B) {
				return io.ErrUnexpectedEOF
			}

			copy(c.Signer[:], r.B)
			if canoto.IsZero(c.Signer) {
				return canoto.ErrZeroValue
			}
			r.B = r.B[expectedLength:]
		case 2:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			var length int64
			if err := canoto.ReadInt(&r, &length); err != nil {
				return err
			}

			const (
				expectedLength      = len(c.Signature)
				expectedLengthInt64 = int64(expectedLength)
			)
			if length != expectedLengthInt64 {
				return canoto.ErrInvalidLength
			}
			if expectedLength > len(r.B) {
				return io.ErrUnexpectedEOF
			}

			copy(c.Signature[:], r.B)
			if canoto.IsZero(c.Signature) {
				return canoto.ErrZeroValue
			}
			r.B = r.B[expectedLength:]
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
func (c *SECP256R1) ValidCanoto() bool {
	if c == nil {
		return true
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *SECP256R1) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	if !canoto.IsZero(c.Signer) {
		c.canotoData.size += len(canoto__SECP256R1__Signer__tag) + canoto.SizeBytes(c.Signer[:])
	}
	if !canoto.IsZero(c.Signature) {
		c.canotoData.size += len(canoto__SECP256R1__Signature__tag) + canoto.SizeBytes(c.Signature[:])
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *SECP256R1) CachedCanotoSize() int {
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
func (c *SECP256R1) MarshalCanoto() []byte {
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
func (c *SECP256R1) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	if !canoto.IsZero(c.Signer) {
		canoto.Append(&w, canoto__SECP256R1__Signer__tag)
		canoto.AppendBytes(&w, c.Signer[:])
	}
	if !canoto.IsZero(c.Signature) {
		canoto.Append(&w, canoto__SECP256R1__Signature__tag)
		canoto.AppendBytes(&w, c.Signature[:])
	}
	return w
}

const (
	canoto__SECP256R1Factory__priv__tag = "\x0a" // canoto.Tag(1, canoto.Len)
)

type canotoData_SECP256R1Factory struct {
	// Enforce noCopy before atomic usage.
	// See https://github.com/StephenButtolph/canoto/pull/32
	_ atomic.Int64

	size int
}

// MakeCanoto creates a new empty value.
func (*SECP256R1Factory) MakeCanoto() *SECP256R1Factory {
	return new(SECP256R1Factory)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *SECP256R1Factory) UnmarshalCanoto(bytes []byte) error {
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
func (c *SECP256R1Factory) UnmarshalCanotoFrom(r canoto.Reader) error {
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

			var length int64
			if err := canoto.ReadInt(&r, &length); err != nil {
				return err
			}

			const (
				expectedLength      = len(c.priv)
				expectedLengthInt64 = int64(expectedLength)
			)
			if length != expectedLengthInt64 {
				return canoto.ErrInvalidLength
			}
			if expectedLength > len(r.B) {
				return io.ErrUnexpectedEOF
			}

			copy(c.priv[:], r.B)
			if canoto.IsZero(c.priv) {
				return canoto.ErrZeroValue
			}
			r.B = r.B[expectedLength:]
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
func (c *SECP256R1Factory) ValidCanoto() bool {
	if c == nil {
		return true
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *SECP256R1Factory) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	if !canoto.IsZero(c.priv) {
		c.canotoData.size += len(canoto__SECP256R1Factory__priv__tag) + canoto.SizeBytes(c.priv[:])
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *SECP256R1Factory) CachedCanotoSize() int {
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
func (c *SECP256R1Factory) MarshalCanoto() []byte {
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
func (c *SECP256R1Factory) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	if !canoto.IsZero(c.priv) {
		canoto.Append(&w, canoto__SECP256R1Factory__priv__tag)
		canoto.AppendBytes(&w, c.priv[:])
	}
	return w
}

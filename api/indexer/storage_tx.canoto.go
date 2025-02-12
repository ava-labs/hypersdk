// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.10.0
// source: api/indexer/storage_tx.go

package indexer

import (
	"io"
	"sync/atomic"
	"unicode/utf8"

	"github.com/ava-labs/hypersdk/internal/canoto"
)

// Ensure that unused imports do not error
var (
	_ atomic.Int64

	_ = io.ErrUnexpectedEOF
	_ = utf8.ValidString
)

const (
	canoto__storageTx__Timestamp__tag = "\x08" // canoto.Tag(1, canoto.Varint)
	canoto__storageTx__Success__tag   = "\x10" // canoto.Tag(2, canoto.Varint)
	canoto__storageTx__Units__tag     = "\x1a" // canoto.Tag(3, canoto.Len)
	canoto__storageTx__Fee__tag       = "\x20" // canoto.Tag(4, canoto.Varint)
	canoto__storageTx__Outputs__tag   = "\x2a" // canoto.Tag(5, canoto.Len)
	canoto__storageTx__Error__tag     = "\x32" // canoto.Tag(6, canoto.Len)
	canoto__storageTx__TxBytes__tag   = "\x3a" // canoto.Tag(7, canoto.Len)
)

type canotoData_storageTx struct {
	// Enforce noCopy before atomic usage.
	// See https://github.com/StephenButtolph/canoto/pull/32
	_ atomic.Int64

	size int
}

// MakeCanoto creates a new empty value.
func (*storageTx) MakeCanoto() *storageTx {
	return new(storageTx)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *storageTx) UnmarshalCanoto(bytes []byte) error {
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
func (c *storageTx) UnmarshalCanotoFrom(r canoto.Reader) error {
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

			if err := canoto.ReadInt(&r, &c.Timestamp); err != nil {
				return err
			}
			if canoto.IsZero(c.Timestamp) {
				return canoto.ErrZeroValue
			}
		case 2:
			if wireType != canoto.Varint {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadBool(&r, &c.Success); err != nil {
				return err
			}
			if canoto.IsZero(c.Success) {
				return canoto.ErrZeroValue
			}
		case 3:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadBytes(&r, &c.Units); err != nil {
				return err
			}
			if len(c.Units) == 0 {
				return canoto.ErrZeroValue
			}
		case 4:
			if wireType != canoto.Varint {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadInt(&r, &c.Fee); err != nil {
				return err
			}
			if canoto.IsZero(c.Fee) {
				return canoto.ErrZeroValue
			}
		case 5:
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

			count, err := canoto.CountBytes(r.B, canoto__storageTx__Outputs__tag)
			if err != nil {
				return err
			}
			c.Outputs = canoto.MakeSlice(c.Outputs, 1+count)

			r.B = remainingBytes
			if err := canoto.ReadBytes(&r, &c.Outputs[0]); err != nil {
				return err
			}
			for i := range count {
				r.B = r.B[len(canoto__storageTx__Outputs__tag):]
				if err := canoto.ReadBytes(&r, &c.Outputs[1+i]); err != nil {
					return err
				}
			}
		case 6:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadString(&r, &c.Error); err != nil {
				return err
			}
			if len(c.Error) == 0 {
				return canoto.ErrZeroValue
			}
		case 7:
			if wireType != canoto.Len {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadBytes(&r, &c.TxBytes); err != nil {
				return err
			}
			if len(c.TxBytes) == 0 {
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
func (c *storageTx) ValidCanoto() bool {
	if c == nil {
		return true
	}
	if !utf8.ValidString(string(c.Error)) {
		return false
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *storageTx) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	if !canoto.IsZero(c.Timestamp) {
		c.canotoData.size += len(canoto__storageTx__Timestamp__tag) + canoto.SizeInt(c.Timestamp)
	}
	if !canoto.IsZero(c.Success) {
		c.canotoData.size += len(canoto__storageTx__Success__tag) + canoto.SizeBool
	}
	if len(c.Units) != 0 {
		c.canotoData.size += len(canoto__storageTx__Units__tag) + canoto.SizeBytes(c.Units)
	}
	if !canoto.IsZero(c.Fee) {
		c.canotoData.size += len(canoto__storageTx__Fee__tag) + canoto.SizeInt(c.Fee)
	}
	for _, v := range c.Outputs {
		c.canotoData.size += len(canoto__storageTx__Outputs__tag) + canoto.SizeBytes(v)
	}
	if len(c.Error) != 0 {
		c.canotoData.size += len(canoto__storageTx__Error__tag) + canoto.SizeBytes(c.Error)
	}
	if len(c.TxBytes) != 0 {
		c.canotoData.size += len(canoto__storageTx__TxBytes__tag) + canoto.SizeBytes(c.TxBytes)
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *storageTx) CachedCanotoSize() int {
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
func (c *storageTx) MarshalCanoto() []byte {
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
func (c *storageTx) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	if !canoto.IsZero(c.Timestamp) {
		canoto.Append(&w, canoto__storageTx__Timestamp__tag)
		canoto.AppendInt(&w, c.Timestamp)
	}
	if !canoto.IsZero(c.Success) {
		canoto.Append(&w, canoto__storageTx__Success__tag)
		canoto.AppendBool(&w, true)
	}
	if len(c.Units) != 0 {
		canoto.Append(&w, canoto__storageTx__Units__tag)
		canoto.AppendBytes(&w, c.Units)
	}
	if !canoto.IsZero(c.Fee) {
		canoto.Append(&w, canoto__storageTx__Fee__tag)
		canoto.AppendInt(&w, c.Fee)
	}
	for _, v := range c.Outputs {
		canoto.Append(&w, canoto__storageTx__Outputs__tag)
		canoto.AppendBytes(&w, v)
	}
	if len(c.Error) != 0 {
		canoto.Append(&w, canoto__storageTx__Error__tag)
		canoto.AppendBytes(&w, c.Error)
	}
	if len(c.TxBytes) != 0 {
		canoto.Append(&w, canoto__storageTx__TxBytes__tag)
		canoto.AppendBytes(&w, c.TxBytes)
	}
	return w
}

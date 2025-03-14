// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.10.0
// source: pubsub/messages.go

package pubsub

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
	canoto__BatchMessage__Messages__tag = "\x0a" // canoto.Tag(1, canoto.Len)
)

type canotoData_BatchMessage struct {
	// Enforce noCopy before atomic usage.
	// See https://github.com/StephenButtolph/canoto/pull/32
	_ atomic.Int64

	size int
}

// MakeCanoto creates a new empty value.
func (*BatchMessage) MakeCanoto() *BatchMessage {
	return new(BatchMessage)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *BatchMessage) UnmarshalCanoto(bytes []byte) error {
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
func (c *BatchMessage) UnmarshalCanotoFrom(r canoto.Reader) error {
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

			remainingBytes := r.B
			originalUnsafe := r.Unsafe
			r.Unsafe = true
			err := canoto.ReadBytes(&r, new([]byte))
			r.Unsafe = originalUnsafe
			if err != nil {
				return err
			}

			count, err := canoto.CountBytes(r.B, canoto__BatchMessage__Messages__tag)
			if err != nil {
				return err
			}
			c.Messages = canoto.MakeSlice(c.Messages, 1+count)

			r.B = remainingBytes
			if err := canoto.ReadBytes(&r, &c.Messages[0]); err != nil {
				return err
			}
			for i := range count {
				r.B = r.B[len(canoto__BatchMessage__Messages__tag):]
				if err := canoto.ReadBytes(&r, &c.Messages[1+i]); err != nil {
					return err
				}
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
func (c *BatchMessage) ValidCanoto() bool {
	if c == nil {
		return true
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *BatchMessage) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	for _, v := range c.Messages {
		c.canotoData.size += len(canoto__BatchMessage__Messages__tag) + canoto.SizeBytes(v)
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *BatchMessage) CachedCanotoSize() int {
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
func (c *BatchMessage) MarshalCanoto() []byte {
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
func (c *BatchMessage) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	for _, v := range c.Messages {
		canoto.Append(&w, canoto__BatchMessage__Messages__tag)
		canoto.AppendBytes(&w, v)
	}
	return w
}

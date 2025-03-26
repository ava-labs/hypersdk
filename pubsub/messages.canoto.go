// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.15.0
// source: messages.go

package pubsub

import (
	"io"
	"reflect"
	"sync/atomic"

	"github.com/StephenButtolph/canoto"
)

// Ensure that unused imports do not error
var (
	_ atomic.Uint64

	_ = io.ErrUnexpectedEOF
)

const (
	canoto__BatchMessage__Messages__tag = "\x0a" // canoto.Tag(1, canoto.Len)
)

type canotoData_BatchMessage struct {
	size atomic.Uint64
}

// CanotoSpec returns the specification of this canoto message.
func (*BatchMessage) CanotoSpec(...reflect.Type) *canoto.Spec {
	s := &canoto.Spec{
		Name: "BatchMessage",
		Fields: []canoto.FieldType{
			{
				FieldNumber: 1,
				Name:        "Messages",
				Repeated:    true,
				OneOf:       "",
				TypeBytes:   true,
			},
		},
	}
	s.CalculateCanotoCache()
	return s
}

// MakeCanoto creates a new empty value.
func (*BatchMessage) MakeCanoto() *BatchMessage {
	return new(BatchMessage)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// During parsing, the canoto cache is saved.
func (c *BatchMessage) UnmarshalCanoto(bytes []byte) error {
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
func (c *BatchMessage) UnmarshalCanotoFrom(r canoto.Reader) error {
	// Zero the struct before unmarshaling.
	*c = BatchMessage{}
	c.canotoData.size.Store(uint64(len(r.B)))

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

			// Skip the first entry because we have already stripped the tag.
			remainingBytes := r.B
			originalUnsafe := r.Unsafe
			r.Unsafe = true
			if err := canoto.ReadBytes(&r, new([]byte)); err != nil {
				return err
			}
			r.Unsafe = originalUnsafe

			// Count the number of additional entries after the first entry.
			countMinus1, err := canoto.CountBytes(r.B, canoto__BatchMessage__Messages__tag)
			if err != nil {
				return err
			}
			c.Messages = canoto.MakeSlice(c.Messages, countMinus1+1)

			// Read the first entry manually because the tag is still already
			// stripped.
			r.B = remainingBytes
			if err := canoto.ReadBytes(&r, &c.Messages[0]); err != nil {
				return err
			}

			// Read the rest of the entries, stripping the tag each time.
			for i := range countMinus1 {
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
func (c *BatchMessage) CalculateCanotoCache() {
	if c == nil {
		return
	}
	var size uint64
	for _, v := range c.Messages {
		size += uint64(len(canoto__BatchMessage__Messages__tag)) + canoto.SizeBytes(v)
	}
	c.canotoData.size.Store(size)
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *BatchMessage) CachedCanotoSize() uint64 {
	if c == nil {
		return 0
	}
	return c.canotoData.size.Load()
}

// MarshalCanoto returns the Canoto representation of this struct.
//
// It is assumed that this struct is ValidCanoto.
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

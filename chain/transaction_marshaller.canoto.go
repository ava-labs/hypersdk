// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.15.0
// source: transaction_marshaller.go

package chain

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
	canoto__BatchedTransactions__Transactions__tag = "\x0a" // canoto.Tag(1, canoto.Len)
)

type canotoData_BatchedTransactions struct {
	size atomic.Uint64
}

// CanotoSpec returns the specification of this canoto message.
func (*BatchedTransactions) CanotoSpec(types ...reflect.Type) *canoto.Spec {
	types = append(types, reflect.TypeOf(BatchedTransactions{}))
	var zero BatchedTransactions
	s := &canoto.Spec{
		Name: "BatchedTransactions",
		Fields: []canoto.FieldType{
			canoto.FieldTypeFromField(
				/*type inference:*/ (canoto.MakeEntry(zero.Transactions)),
				/*FieldNumber:   */ 1,
				/*Name:          */ "Transactions",
				/*FixedLength:   */ 0,
				/*Repeated:      */ true,
				/*OneOf:         */ "",
				/*types:         */ types,
			),
		},
	}
	s.CalculateCanotoCache()
	return s
}

// MakeCanoto creates a new empty value.
func (*BatchedTransactions) MakeCanoto() *BatchedTransactions {
	return new(BatchedTransactions)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// During parsing, the canoto cache is saved.
func (c *BatchedTransactions) UnmarshalCanoto(bytes []byte) error {
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
func (c *BatchedTransactions) UnmarshalCanotoFrom(r canoto.Reader) error {
	// Zero the struct before unmarshaling.
	*c = BatchedTransactions{}
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
			countMinus1, err := canoto.CountBytes(r.B, canoto__BatchedTransactions__Transactions__tag)
			if err != nil {
				return err
			}

			c.Transactions = canoto.MakeSlice(c.Transactions, countMinus1+1)
			if len(msgBytes) != 0 {
				remainingBytes := r.B
				r.B = msgBytes
				c.Transactions[0] = canoto.MakePointer(c.Transactions[0])
				if err := (c.Transactions[0]).UnmarshalCanotoFrom(r); err != nil {
					return err
				}
				r.B = remainingBytes
			}

			// Read the rest of the entries, stripping the tag each time.
			for i := range countMinus1 {
				r.B = r.B[len(canoto__BatchedTransactions__Transactions__tag):]
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
				c.Transactions[1+i] = canoto.MakePointer(c.Transactions[1+i])
				if err := (c.Transactions[1+i]).UnmarshalCanotoFrom(r); err != nil {
					return err
				}
				r.B = remainingBytes
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
func (c *BatchedTransactions) ValidCanoto() bool {
	if c == nil {
		return true
	}
	for i := range c.Transactions {
		if c.Transactions[i] != nil && !(c.Transactions[i]).ValidCanoto() {
			return false
		}
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
func (c *BatchedTransactions) CalculateCanotoCache() {
	if c == nil {
		return
	}
	var size uint64
	for i := range c.Transactions {
		var fieldSize uint64
		if c.Transactions[i] != nil {
			(c.Transactions[i]).CalculateCanotoCache()
			fieldSize = (c.Transactions[i]).CachedCanotoSize()
		}
		size += uint64(len(canoto__BatchedTransactions__Transactions__tag)) + canoto.SizeUint(fieldSize) + fieldSize
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
func (c *BatchedTransactions) CachedCanotoSize() uint64 {
	if c == nil {
		return 0
	}
	return c.canotoData.size.Load()
}

// MarshalCanoto returns the Canoto representation of this struct.
//
// It is assumed that this struct is ValidCanoto.
func (c *BatchedTransactions) MarshalCanoto() []byte {
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
func (c *BatchedTransactions) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	for i := range c.Transactions {
		canoto.Append(&w, canoto__BatchedTransactions__Transactions__tag)
		var fieldSize uint64
		if c.Transactions[i] != nil {
			fieldSize = (c.Transactions[i]).CachedCanotoSize()
		}
		canoto.AppendUint(&w, fieldSize)
		if fieldSize != 0 {
			w = (c.Transactions[i]).MarshalCanotoInto(w)
		}
	}
	return w
}

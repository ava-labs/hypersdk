// Code generated by canoto. DO NOT EDIT.
// versions:
// 	canoto v0.9.1
// source: block.go

package dsmr

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
	canoto__UnsignedChunk__Producer__tag    = "\x0a" // canoto.Tag(1, canoto.Len)
	canoto__UnsignedChunk__Beneficiary__tag = "\x12" // canoto.Tag(2, canoto.Len)
	canoto__UnsignedChunk__Expiry__tag      = "\x18" // canoto.Tag(3, canoto.Varint)
	canoto__UnsignedChunk__Txs__tag         = "\x22" // canoto.Tag(4, canoto.Len)
)

type canotoData_UnsignedChunk struct {
	size int
}

// MakeCanoto creates a new empty value.
func (*UnsignedChunk[T1]) MakeCanoto() *UnsignedChunk[T1] {
	return new(UnsignedChunk[T1])
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *UnsignedChunk[T1]) UnmarshalCanoto(bytes []byte) error {
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
func (c *UnsignedChunk[T1]) UnmarshalCanotoFrom(r canoto.Reader) error {
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
				expectedLength      = len(c.Producer)
				expectedLengthInt64 = int64(expectedLength)
			)
			if length != expectedLengthInt64 {
				return canoto.ErrInvalidLength
			}
			if expectedLength > len(r.B) {
				return io.ErrUnexpectedEOF
			}

			copy(c.Producer[:], r.B)
			if canoto.IsZero(c.Producer) {
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
				expectedLength      = len(c.Beneficiary)
				expectedLengthInt64 = int64(expectedLength)
			)
			if length != expectedLengthInt64 {
				return canoto.ErrInvalidLength
			}
			if expectedLength > len(r.B) {
				return io.ErrUnexpectedEOF
			}

			copy(c.Beneficiary[:], r.B)
			if canoto.IsZero(c.Beneficiary) {
				return canoto.ErrZeroValue
			}
			r.B = r.B[expectedLength:]
		case 3:
			if wireType != canoto.Varint {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadInt(&r, &c.Expiry); err != nil {
				return err
			}
			if canoto.IsZero(c.Expiry) {
				return canoto.ErrZeroValue
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
			count, err := canoto.CountBytes(remainingBytes, canoto__UnsignedChunk__Txs__tag)
			if err != nil {
				return err
			}

			c.Txs = canoto.MakeSlice(c.Txs, 1+count)
			if len(msgBytes) != 0 {
				r.B = msgBytes
				c.Txs[0] = c.Txs[0].MakeCanoto()
				err = c.Txs[0].UnmarshalCanotoFrom(r)
				r.B = remainingBytes
				if err != nil {
					return err
				}
			}

			for i := range count {
				r.B = r.B[len(canoto__UnsignedChunk__Txs__tag):]
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
				c.Txs[1+i] = c.Txs[1+i].MakeCanoto()
				err = c.Txs[1+i].UnmarshalCanotoFrom(r)
				r.B = remainingBytes
				if err != nil {
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
func (c *UnsignedChunk[T1]) ValidCanoto() bool {
	if c == nil {
		return true
	}
	for i := range c.Txs {
		if !c.Txs[i].ValidCanoto() {
			return false
		}
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *UnsignedChunk[T1]) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	if !canoto.IsZero(c.Producer) {
		c.canotoData.size += len(canoto__UnsignedChunk__Producer__tag) + canoto.SizeBytes(c.Producer[:])
	}
	if !canoto.IsZero(c.Beneficiary) {
		c.canotoData.size += len(canoto__UnsignedChunk__Beneficiary__tag) + canoto.SizeBytes(c.Beneficiary[:])
	}
	if !canoto.IsZero(c.Expiry) {
		c.canotoData.size += len(canoto__UnsignedChunk__Expiry__tag) + canoto.SizeInt(c.Expiry)
	}
	for i := range c.Txs {
		c.Txs[i].CalculateCanotoCache()
		fieldSize := c.Txs[i].CachedCanotoSize()
		c.canotoData.size += len(canoto__UnsignedChunk__Txs__tag) + canoto.SizeInt(int64(fieldSize)) + fieldSize
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *UnsignedChunk[T1]) CachedCanotoSize() int {
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
func (c *UnsignedChunk[T1]) MarshalCanoto() []byte {
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
func (c *UnsignedChunk[T1]) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	if !canoto.IsZero(c.Producer) {
		canoto.Append(&w, canoto__UnsignedChunk__Producer__tag)
		canoto.AppendBytes(&w, c.Producer[:])
	}
	if !canoto.IsZero(c.Beneficiary) {
		canoto.Append(&w, canoto__UnsignedChunk__Beneficiary__tag)
		canoto.AppendBytes(&w, c.Beneficiary[:])
	}
	if !canoto.IsZero(c.Expiry) {
		canoto.Append(&w, canoto__UnsignedChunk__Expiry__tag)
		canoto.AppendInt(&w, c.Expiry)
	}
	for i := range c.Txs {
		canoto.Append(&w, canoto__UnsignedChunk__Txs__tag)
		fieldSize := c.Txs[i].CachedCanotoSize()
		canoto.AppendInt(&w, int64(fieldSize))
		if fieldSize != 0 {
			w = c.Txs[i].MarshalCanotoInto(w)
		}
	}
	return w
}

const (
	canoto__Chunk__UnsignedChunk__tag = "\x0a" // canoto.Tag(1, canoto.Len)
	canoto__Chunk__Signer__tag        = "\x12" // canoto.Tag(2, canoto.Len)
	canoto__Chunk__Signature__tag     = "\x1a" // canoto.Tag(3, canoto.Len)
)

type canotoData_Chunk struct {
	size int
}

// MakeCanoto creates a new empty value.
func (*Chunk[T1]) MakeCanoto() *Chunk[T1] {
	return new(Chunk[T1])
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *Chunk[T1]) UnmarshalCanoto(bytes []byte) error {
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
func (c *Chunk[T1]) UnmarshalCanotoFrom(r canoto.Reader) error {
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
			if len(msgBytes) == 0 {
				return canoto.ErrZeroValue
			}

			remainingBytes := r.B
			r.B = msgBytes
			err = (&c.UnsignedChunk).UnmarshalCanotoFrom(r)
			r.B = remainingBytes
			if err != nil {
				return err
			}
		case 2:
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
		case 3:
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
func (c *Chunk[T1]) ValidCanoto() bool {
	if c == nil {
		return true
	}
	if !(&c.UnsignedChunk).ValidCanoto() {
		return false
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *Chunk[T1]) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	(&c.UnsignedChunk).CalculateCanotoCache()
	if fieldSize := (&c.UnsignedChunk).CachedCanotoSize(); fieldSize != 0 {
		c.canotoData.size += len(canoto__Chunk__UnsignedChunk__tag) + canoto.SizeInt(int64(fieldSize)) + fieldSize
	}
	if !canoto.IsZero(c.Signer) {
		c.canotoData.size += len(canoto__Chunk__Signer__tag) + canoto.SizeBytes(c.Signer[:])
	}
	if !canoto.IsZero(c.Signature) {
		c.canotoData.size += len(canoto__Chunk__Signature__tag) + canoto.SizeBytes(c.Signature[:])
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *Chunk[T1]) CachedCanotoSize() int {
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
func (c *Chunk[T1]) MarshalCanoto() []byte {
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
func (c *Chunk[T1]) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	if fieldSize := (&c.UnsignedChunk).CachedCanotoSize(); fieldSize != 0 {
		canoto.Append(&w, canoto__Chunk__UnsignedChunk__tag)
		canoto.AppendInt(&w, int64(fieldSize))
		w = (&c.UnsignedChunk).MarshalCanotoInto(w)
	}
	if !canoto.IsZero(c.Signer) {
		canoto.Append(&w, canoto__Chunk__Signer__tag)
		canoto.AppendBytes(&w, c.Signer[:])
	}
	if !canoto.IsZero(c.Signature) {
		canoto.Append(&w, canoto__Chunk__Signature__tag)
		canoto.AppendBytes(&w, c.Signature[:])
	}
	return w
}

const (
	canoto__BlockHeader__ParentID__tag  = "\x0a" // canoto.Tag(1, canoto.Len)
	canoto__BlockHeader__Height__tag    = "\x10" // canoto.Tag(2, canoto.Varint)
	canoto__BlockHeader__Timestamp__tag = "\x18" // canoto.Tag(3, canoto.Varint)
)

type canotoData_BlockHeader struct {
	size int
}

// MakeCanoto creates a new empty value.
func (*BlockHeader) MakeCanoto() *BlockHeader {
	return new(BlockHeader)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *BlockHeader) UnmarshalCanoto(bytes []byte) error {
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
func (c *BlockHeader) UnmarshalCanotoFrom(r canoto.Reader) error {
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
				expectedLength      = len(c.ParentID)
				expectedLengthInt64 = int64(expectedLength)
			)
			if length != expectedLengthInt64 {
				return canoto.ErrInvalidLength
			}
			if expectedLength > len(r.B) {
				return io.ErrUnexpectedEOF
			}

			copy(c.ParentID[:], r.B)
			if canoto.IsZero(c.ParentID) {
				return canoto.ErrZeroValue
			}
			r.B = r.B[expectedLength:]
		case 2:
			if wireType != canoto.Varint {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadInt(&r, &c.Height); err != nil {
				return err
			}
			if canoto.IsZero(c.Height) {
				return canoto.ErrZeroValue
			}
		case 3:
			if wireType != canoto.Varint {
				return canoto.ErrUnexpectedWireType
			}

			if err := canoto.ReadInt(&r, &c.Timestamp); err != nil {
				return err
			}
			if canoto.IsZero(c.Timestamp) {
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
func (c *BlockHeader) ValidCanoto() bool {
	if c == nil {
		return true
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *BlockHeader) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	if !canoto.IsZero(c.ParentID) {
		c.canotoData.size += len(canoto__BlockHeader__ParentID__tag) + canoto.SizeBytes(c.ParentID[:])
	}
	if !canoto.IsZero(c.Height) {
		c.canotoData.size += len(canoto__BlockHeader__Height__tag) + canoto.SizeInt(c.Height)
	}
	if !canoto.IsZero(c.Timestamp) {
		c.canotoData.size += len(canoto__BlockHeader__Timestamp__tag) + canoto.SizeInt(c.Timestamp)
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *BlockHeader) CachedCanotoSize() int {
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
func (c *BlockHeader) MarshalCanoto() []byte {
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
func (c *BlockHeader) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	if !canoto.IsZero(c.ParentID) {
		canoto.Append(&w, canoto__BlockHeader__ParentID__tag)
		canoto.AppendBytes(&w, c.ParentID[:])
	}
	if !canoto.IsZero(c.Height) {
		canoto.Append(&w, canoto__BlockHeader__Height__tag)
		canoto.AppendInt(&w, c.Height)
	}
	if !canoto.IsZero(c.Timestamp) {
		canoto.Append(&w, canoto__BlockHeader__Timestamp__tag)
		canoto.AppendInt(&w, c.Timestamp)
	}
	return w
}

const (
	canoto__Block__ChunkCerts__tag = "\x0a" // canoto.Tag(1, canoto.Len)
)

type canotoData_Block struct {
	size int
}

// MakeCanoto creates a new empty value.
func (*Block) MakeCanoto() *Block {
	return new(Block)
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *Block) UnmarshalCanoto(bytes []byte) error {
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
func (c *Block) UnmarshalCanotoFrom(r canoto.Reader) error {
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
			count, err := canoto.CountBytes(remainingBytes, canoto__Block__ChunkCerts__tag)
			if err != nil {
				return err
			}

			c.ChunkCerts = canoto.MakeSlice(c.ChunkCerts, 1+count)
			if len(msgBytes) != 0 {
				r.B = msgBytes
				c.ChunkCerts[0] = canoto.MakePointer(c.ChunkCerts[0])
				err = (c.ChunkCerts[0]).UnmarshalCanotoFrom(r)
				r.B = remainingBytes
				if err != nil {
					return err
				}
			}

			for i := range count {
				r.B = r.B[len(canoto__Block__ChunkCerts__tag):]
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
				c.ChunkCerts[1+i] = canoto.MakePointer(c.ChunkCerts[1+i])
				err = (c.ChunkCerts[1+i]).UnmarshalCanotoFrom(r)
				r.B = remainingBytes
				if err != nil {
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
func (c *Block) ValidCanoto() bool {
	if c == nil {
		return true
	}
	for i := range c.ChunkCerts {
		if c.ChunkCerts[i] != nil && !(c.ChunkCerts[i]).ValidCanoto() {
			return false
		}
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *Block) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	for i := range c.ChunkCerts {
		var fieldSize int
		if c.ChunkCerts[i] != nil {
			(c.ChunkCerts[i]).CalculateCanotoCache()
			fieldSize = (c.ChunkCerts[i]).CachedCanotoSize()
		}
		c.canotoData.size += len(canoto__Block__ChunkCerts__tag) + canoto.SizeInt(int64(fieldSize)) + fieldSize
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *Block) CachedCanotoSize() int {
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
func (c *Block) MarshalCanoto() []byte {
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
func (c *Block) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	for i := range c.ChunkCerts {
		canoto.Append(&w, canoto__Block__ChunkCerts__tag)
		var fieldSize int
		if c.ChunkCerts[i] != nil {
			fieldSize = (c.ChunkCerts[i]).CachedCanotoSize()
		}
		canoto.AppendInt(&w, int64(fieldSize))
		if fieldSize != 0 {
			w = (c.ChunkCerts[i]).MarshalCanotoInto(w)
		}
	}
	return w
}

const (
	canoto__ExecutedBlock__Chunks__tag = "\x0a" // canoto.Tag(1, canoto.Len)
)

type canotoData_ExecutedBlock struct {
	size int
}

// MakeCanoto creates a new empty value.
func (*ExecutedBlock[T1]) MakeCanoto() *ExecutedBlock[T1] {
	return new(ExecutedBlock[T1])
}

// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the struct.
//
// OneOf fields are cached during the unmarshaling process.
//
// The struct is not cleared before unmarshaling, any fields not present in the
// bytes will retain their previous values. If a OneOf field was previously
// cached as being set, attempting to unmarshal that OneOf again will return
// canoto.ErrDuplicateOneOf.
func (c *ExecutedBlock[T1]) UnmarshalCanoto(bytes []byte) error {
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
func (c *ExecutedBlock[T1]) UnmarshalCanotoFrom(r canoto.Reader) error {
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
			count, err := canoto.CountBytes(remainingBytes, canoto__ExecutedBlock__Chunks__tag)
			if err != nil {
				return err
			}

			c.Chunks = canoto.MakeSlice(c.Chunks, 1+count)
			if len(msgBytes) != 0 {
				r.B = msgBytes
				err = (&c.Chunks[0]).UnmarshalCanotoFrom(r)
				r.B = remainingBytes
				if err != nil {
					return err
				}
			}

			for i := range count {
				r.B = r.B[len(canoto__ExecutedBlock__Chunks__tag):]
				r.Unsafe = true
				err := canoto.ReadBytes(&r, &msgBytes)
				r.Unsafe = originalUnsafe
				if err != nil {
					return err
				}

				if len(msgBytes) != 0 {
					remainingBytes := r.B
					r.B = msgBytes
					err = (&c.Chunks[1+i]).UnmarshalCanotoFrom(r)
					r.B = remainingBytes
					if err != nil {
						return err
					}
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
func (c *ExecutedBlock[T1]) ValidCanoto() bool {
	if c == nil {
		return true
	}
	for i := range c.Chunks {
		if !(&c.Chunks[i]).ValidCanoto() {
			return false
		}
	}
	return true
}

// CalculateCanotoCache populates size and OneOf caches based on the current
// values in the struct.
//
// It is not safe to call this function concurrently.
func (c *ExecutedBlock[T1]) CalculateCanotoCache() {
	if c == nil {
		return
	}
	c.canotoData.size = 0
	for i := range c.Chunks {
		(&c.Chunks[i]).CalculateCanotoCache()
		fieldSize := (&c.Chunks[i]).CachedCanotoSize()
		c.canotoData.size += len(canoto__ExecutedBlock__Chunks__tag) + canoto.SizeInt(int64(fieldSize)) + fieldSize
	}
}

// CachedCanotoSize returns the previously calculated size of the Canoto
// representation from CalculateCanotoCache.
//
// If CalculateCanotoCache has not yet been called, it will return 0.
//
// If the struct has been modified since the last call to CalculateCanotoCache,
// the returned size may be incorrect.
func (c *ExecutedBlock[T1]) CachedCanotoSize() int {
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
func (c *ExecutedBlock[T1]) MarshalCanoto() []byte {
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
func (c *ExecutedBlock[T1]) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	if c == nil {
		return w
	}
	for i := range c.Chunks {
		canoto.Append(&w, canoto__ExecutedBlock__Chunks__tag)
		canoto.AppendInt(&w, int64((&c.Chunks[i]).CachedCanotoSize()))
		w = (&c.Chunks[i]).MarshalCanotoInto(w)
	}
	return w
}

package memory

import "math"

type Ptr uint32

type Slice interface {
	Ptr() Ptr
	Len() uint32
}

type slice struct {
	ptr Ptr
	len uint32
}

func (s slice) Ptr() Ptr {
	return s.ptr
}
func (s slice) Len() uint32 {
	return s.len
}

type Memory interface {
	Load(slice Slice) ([]byte, error)
	Write(ptr Ptr, data []byte) error
	alloc(len uint32) (Ptr, error)
	Grow(delta uint32) (uint32, error)
	Capacity() (uint32, error)
}

// WriteBytes is a helper function that allocates memory and writes the given
// bytes to the memory returning the offset.
func WriteBytes(m Memory, buf []byte) (Slice, error) {
	if len(buf) > math.MaxUint32 {
		return nil, ErrOverflow
	}
	ptr, err := m.alloc(uint32(len(buf)))
	if err != nil {
		return nil, err
	}
	err = m.Write(ptr, buf)
	if err != nil {
		return nil, err
	}

	return slice{ptr: ptr, len: uint32(len(buf))}, nil
}

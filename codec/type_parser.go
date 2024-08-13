package codec

import "github.com/ava-labs/hypersdk/consts"

type decoder[T any] struct {
	f func(*Packer) (T, error)
}

// The number of types is limited to 255.
type TypeParser[T any] struct {
	typeToIndex    map[string]uint8
	indexToDecoder map[uint8]*decoder[T]
}

// NewTypeParser returns an instance of a Typeparser with generic type [T].
func NewTypeParser[T any]() *TypeParser[T] {
	return &TypeParser[T]{
		typeToIndex:    map[string]uint8{},
		indexToDecoder: map[uint8]*decoder[T]{},
	}
}

// Register registers a new type into TypeParser [p]. Registers the type by using
// the string representation of [o], and sets the decoder of that index to [f].
// Returns an error if [o] has already been registered or the TypeParser is full.
func (p *TypeParser[T]) Register(id uint8, f func(*Packer) (T, error)) error {
	if len(p.indexToDecoder) == int(consts.MaxUint8)+1 {
		return ErrTooManyItems
	}
	if _, ok := p.indexToDecoder[id]; ok {
		return ErrDuplicateItem
	}
	p.indexToDecoder[id] = &decoder[T]{f: f}
	return nil
}

// LookupIndex returns the decoder function and success of lookup of [index]
// from Typeparser [p].
func (p *TypeParser[T]) LookupIndex(index uint8) (func(*Packer) (T, error), bool) {
	d, ok := p.indexToDecoder[index]
	if ok {
		return d.f, true
	}
	return nil, false
}

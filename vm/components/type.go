package components

import "github.com/ava-labs/hypersdk/codec"

var _ codec.Typed = (*GenericType)(nil)

type GenericType struct {
	// Result is the underlying result
	g interface{}
	// type ID of the result
	TypeID uint8
}

func NewGenericType(gtype interface{}, typeID uint8) *GenericType {
	return &GenericType{
		g: gtype,
		TypeID: typeID,
	}
}

func (g GenericType) GetTypeID() uint8 {
	return g.TypeID
}


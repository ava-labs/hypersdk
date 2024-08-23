package codeccopy

import "github.com/ava-labs/avalanchego/utils/wrappers"

// Codec marshals and unmarshals
type Codec interface {
	MarshalInto(interface{}, *wrappers.Packer) error
	Unmarshal([]byte, interface{}) error

	// Returns the size, in bytes, of [value] when it's marshaled
	Size(value interface{}) (int, error)
}

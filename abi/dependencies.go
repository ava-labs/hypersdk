package abi

import "github.com/ava-labs/hypersdk/codec"

type ActionPair struct {
	Input  codec.Typed
	Output interface{}
}

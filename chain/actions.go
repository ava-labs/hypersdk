// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

func GetSize(item interface{}) (int, error) {
	if actionWithSize, ok := item.(Marshaler); ok {
		return actionWithSize.Size(), nil
	}
	return codec.LinearCodec.Size(item)
}

func marshalInto(item interface{}, p *codec.Packer) error {
	if actionWithMarshal, ok := item.(Marshaler); ok {
		actionWithMarshal.Marshal(p)
		return nil
	}
	return codec.LinearCodec.MarshalInto(item, p.Packer)
}
func Marshal(v interface{}) ([]byte, error) {
	size, err := GetSize(v)
	if err != nil {
		return nil, err
	}
	p := codec.NewWriter(size, consts.NetworkSizeLimit)
	if err := marshalInto(v, p); err != nil {
		return nil, err
	}
	return p.Bytes(), nil
}

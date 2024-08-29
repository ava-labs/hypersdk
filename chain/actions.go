// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import "github.com/ava-labs/hypersdk/codec"

func getActionSize(action Action) (int, error) {
	if actionWithSize, ok := action.(Marshaler); ok {
		return actionWithSize.Size(), nil
	}
	return codec.LinearCodec.Size(action)
}

func marshalActionInto(action Action, p *codec.Packer) error {
	if actionWithMarshal, ok := action.(Marshaler); ok {
		actionWithMarshal.Marshal(p)
		return nil
	}
	return codec.LinearCodec.MarshalInto(action, p.Packer)
}

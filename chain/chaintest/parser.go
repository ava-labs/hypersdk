// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"errors"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

func NewTestParser() *chain.TxTypeParser {
	actionCodec := codec.NewTypeParser[chain.Action]()
	authCodec := codec.NewTypeParser[chain.Auth]()
	outputCodec := codec.NewTypeParser[codec.Typed]()

	err := errors.Join(
		actionCodec.Register(&TestAction{}, nil),
		authCodec.Register(&TestAuth{}, UnmarshalAuth),
		outputCodec.Register(&TestOutput{}, nil),
	)
	if err != nil {
		panic(err)
	}

	return &chain.TxTypeParser{
		ActionRegistry: actionCodec,
		AuthRegistry:   authCodec,
	}
}

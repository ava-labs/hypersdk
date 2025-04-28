// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"errors"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

func NewTestParser() *chain.TxTypeParser {
	actionCodec := codec.NewCanotoParser[chain.Action]()
	authCodec := codec.NewCanotoParser[chain.Auth]()
	outputCodec := codec.NewCanotoParser[codec.Typed]()

	err := errors.Join(
		actionCodec.Register(&TestAction{}, UnmarshalTestAction),
		authCodec.Register(&TestAuth{}, UnmarshalTestAuth),
		outputCodec.Register(&TestOutput{}, UnmarshalTestOutput),
	)
	if err != nil {
		panic(err)
	}

	return chain.NewTxTypeParser(actionCodec, authCodec)
}

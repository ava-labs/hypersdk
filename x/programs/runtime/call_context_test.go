// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/programs/test"
)

func TestCallContext(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.ProgramLoader{ProgramName: "call_program"},
	).WithProgram(codec.CreateAddress(0, ids.GenerateTestID())).WithFuel(1000000)
	actor := codec.CreateAddress(1, ids.GenerateTestID())

	result, err := r.WithActor(actor).CallProgram(
		ctx,
		&CallInfo{
			FunctionName: "actor_check",
		})
	require.NoError(err)
	require.Equal(actor, test.Into[codec.Address](result))

	result, err = r.WithActor(codec.CreateAddress(2, ids.GenerateTestID())).CallProgram(
		ctx,
		&CallInfo{
			FunctionName: "actor_check",
		})
	require.NoError(err)
	require.NotEqual(actor, test.Into[codec.Address](result))

	result, err = r.WithFuel(0).CallProgram(
		ctx,
		&CallInfo{
			FunctionName: "actor_check",
		})
	require.Equal(wasmtime.OutOfFuel, *err.(*wasmtime.Trap).Code())
	require.Nil(result)
}

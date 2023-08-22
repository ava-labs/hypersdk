// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	_ "embed"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/utils/logging"
)

var (
	//go:embed testdata/get_guest.wasm
	tokenProgramBytes []byte

	// example cost map
	costMap = map[string]uint64{
		"ConstI32 0x0": 1, // initialize i32
		"ConstI32 0x1": 1, // set i32 to value 1
	}
	maxFee uint64 = 3
	log           = logging.NewLogger(
		"",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))
)

// go test -v -run ^TestMeterInsufficientBalance$ github.com/ava-labs/hypersdk/x/programs/runtime
func TestMeterInsufficientBalance(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	storage := NewMockStorage(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	meter := NewMeter(log, maxFee, costMap)
	runtime := New(log, meter, storage)
	err := runtime.Initialize(ctx, tokenProgramBytes, []string{"get"})
	require.NoError(err)

	// first call should pass
	resp, err := runtime.Call(ctx, "get")
	require.NoError(err)
	require.Equal(uint64(1), resp[0])

	// second call should fail due to insufficient balance
	_, err = runtime.Call(ctx, "get")
	require.ErrorIs(err, ErrMeterInsufficientBalance)
}

func TestMeterRuntimeStop(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	storage := NewMockStorage(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	meter := NewMeter(log, maxFee, costMap)
	runtime := New(log, meter, storage)
	err := runtime.Initialize(ctx, tokenProgramBytes, []string{"get"})
	require.NoError(err)

	// shutdown runtime
	err = runtime.Stop(ctx)
	require.NoError(err)

	// meter should be independent to runtime
	err = meter.AddCost(ctx, "ConstI32 0x0")
	require.NoError(err)
}

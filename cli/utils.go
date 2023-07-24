// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"context"
	"time"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	dummyBlockAgeThreshold = 25 * consts.MillisecondsPerSecond
	dummyHeightThreshold   = 3
)

func (*Handler) SubmitDummy(
	ctx context.Context,
	cli *rpc.JSONRPCClient,
	sendAndWait func(context.Context, uint64) error,
) error {
	var (
		logEmitted bool
		txsSent    uint64
	)
	for ctx.Err() == nil {
		_, h, t, err := cli.Accepted(ctx)
		if err != nil {
			return err
		}
		underHeight := h < dummyHeightThreshold
		if underHeight || time.Now().UnixMilli()-t > dummyBlockAgeThreshold {
			if underHeight && !logEmitted {
				utils.Outf(
					"{{yellow}}waiting for snowman++ activation (needed for AWM)...{{/}}\n",
				)
				logEmitted = true
			}
			if err := sendAndWait(ctx, txsSent+1); err != nil {
				return err
			}
			txsSent++
			time.Sleep(750 * time.Millisecond)
			continue
		}
		if logEmitted {
			utils.Outf("{{yellow}}snowman++ activated{{/}}\n")
		}
		return nil
	}
	return ctx.Err()
}

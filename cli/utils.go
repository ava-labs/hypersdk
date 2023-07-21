package cli

import (
	"context"
	"time"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/basevm/actions"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	dummyBlockAgeThreshold = 25 * consts.MillisecondsPerSecond
	dummyHeightThreshold   = 3
)

func submitDummy(
	ctx context.Context,
	cli *rpc.JSONRPCClient,
	tcli *trpc.JSONRPCClient,
	dest crypto.PublicKey,
	factory chain.AuthFactory,
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
			parser, err := tcli.Parser(ctx)
			if err != nil {
				return err
			}
			submit, tx, _, err := cli.GenerateTransaction(ctx, parser, nil, &actions.Transfer{
				To:    dest,
				Value: txsSent + 1, // prevent duplicate txs
			}, factory)
			if err != nil {
				return err
			}
			if err := submit(ctx); err != nil {
				return err
			}
			if _, err := tcli.WaitForTransaction(ctx, tx.ID()); err != nil {
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

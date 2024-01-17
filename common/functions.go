package common

import (
	"context"
	runner_sdk "github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/onsi/gomega"
	"time"
)

// AwaitHealthy waits until the node is healthy, repeatedly polling the health check endpoint
func AwaitHealthy(cli runner_sdk.Client, healthPollInterval time.Duration) {
	for {
		time.Sleep(healthPollInterval)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := cli.Health(ctx)
		cancel() // by default, health will wait to return until healthy
		if err == nil {
			return
		}
		utils.Outf(
			"{{yellow}}waiting for health check to pass:{{/}} %v\n",
			err,
		)
	}
}

// GenerateBlocks generates blocks until either ctx is cancelled or the specified (!= 0) number of blocks is generated.
// if 0 blocks are specified, will just wait until ctx is cancelled.
func GenerateBlocks(
	ctx context.Context,
	cumulativeTxs int,
	blocksToGenerate uint64,
	instances []NodeInstance,
	failOnError bool,
	action chain.Action,
	factory chain.AuthFactory,
) int {
	_, lastHeight, _, err := instances[0].Cli.Accepted(context.Background())
	gomega.Ω(err).Should(gomega.BeNil())
	parser, err := instances[0].VmCli.(RpcClient).Parser(context.Background())
	gomega.Ω(err).Should(gomega.BeNil())
	var targetHeight uint64
	if blocksToGenerate != 0 {
		targetHeight = lastHeight + blocksToGenerate
	}
	for ctx.Err() == nil {
		// Generate transaction
		submit, _, _, err := instances[cumulativeTxs%len(instances)].Cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			action,
			factory,
		)
		if failOnError {
			gomega.Ω(err).Should(gomega.BeNil())
		} else if err != nil {
			utils.Outf(
				"{{yellow}}unable to generate transaction:{{/}} %v\n",
				err,
			)
			time.Sleep(5 * time.Second)
			continue
		}

		// Broadcast transactions
		err = submit(context.Background())
		if failOnError {
			gomega.Ω(err).Should(gomega.BeNil())
		} else if err != nil {
			utils.Outf(
				"{{yellow}}tx broadcast failed:{{/}} %v\n",
				err,
			)
			time.Sleep(5 * time.Second)
			continue
		}
		cumulativeTxs++
		_, height, _, err := instances[0].Cli.Accepted(context.Background())
		if failOnError {
			gomega.Ω(err).Should(gomega.BeNil())
		} else if err != nil {
			utils.Outf(
				"{{yellow}}height lookup failed:{{/}} %v\n",
				err,
			)
			time.Sleep(5 * time.Second)
			continue
		}
		if targetHeight != 0 && height > targetHeight {
			break
		} else if height > lastHeight {
			lastHeight = height
			utils.Outf("{{yellow}}height=%d count=%d{{/}}\n", height, cumulativeTxs)
		}

		// Sleep for a very small amount of time to avoid overloading the
		// network with transactions (can generate very fast)
		time.Sleep(10 * time.Millisecond)
	}
	return cumulativeTxs
}

// AcceptTransaction generates a transaction and waits for it to be accepted by the network
func AcceptTransaction(
	cli *rpc.JSONRPCClient,
	vmCli RpcClient,
	action chain.Action,
	factory chain.AuthFactory,
	requestTimeout time.Duration,
) {
	parser, err := vmCli.Parser(context.Background())
	gomega.Ω(err).Should(gomega.BeNil())
	for {
		// Generate transaction
		unitPrices, err := cli.UnitPrices(context.Background(), false)
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, maxFee, err := cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			action,
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		utils.Outf("{{yellow}}generated transaction{{/}} prices: %+v maxFee: %d\n", unitPrices, maxFee)

		// Broadcast and wait for transaction
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		utils.Outf("{{yellow}}submitted transaction{{/}}\n")
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		success, _, err := vmCli.WaitForTransaction(ctx, tx.ID())
		cancel()
		if err != nil {
			utils.Outf("{{red}}cannot find transaction: %v{{/}}\n", err)
			continue
		}
		gomega.Ω(success).Should(gomega.BeTrue())
		utils.Outf("{{yellow}}found transaction{{/}}\n")
		break
	}
}

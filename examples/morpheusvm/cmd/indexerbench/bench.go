package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/fees"
)

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

const PARENT_TEST_DIR = "/var/tmp/indexer-bench"

func main() {
	// Clean up and create the parent directory if it doesn't exist
	err := os.RemoveAll(PARENT_TEST_DIR)
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(PARENT_TEST_DIR, 0755)
	if err != nil {
		panic(err)
	}
	tempDir := must(os.MkdirTemp(PARENT_TEST_DIR, "bench")) //not tmp to be on the main file system

	parser := chaintest.NewEmptyParser()
	(*parser.ActionRegistry()).Register(&actions.Transfer{}, nil)
	(*parser.OutputRegistry()).Register(&actions.TransferResult{}, nil)
	(*parser.AuthRegistry()).Register(&auth.ED25519{}, nil)

	indexer := must(indexer.NewIndexer(tempDir, parser))

	for i := 0; i < 100; i++ {
		const blockCount = 100000
		const txPerBlock = 1000

		start := time.Now()
		executedBlocks := GenerateExecutedBlocks(parser, ids.Empty, 0, 0, 1, blockCount, txPerBlock)

		// Create a channel to collect results
		results := make(chan error, blockCount)

		// Parallelize the acceptance of blocks
		for _, blk := range executedBlocks {
			go func(blk *chain.ExecutedBlock) {
				results <- indexer.Accept(blk)
			}(blk)
		}

		// Wait for all results
		for i := 0; i < blockCount; i++ {
			if err := <-results; err != nil {
				panic(err)
			}
		}

		fmt.Printf("Processed %s blocks with %s transactions in %s\n", formatNumberWithSuffix(blockCount), formatNumberWithSuffix(txPerBlock*blockCount), time.Since(start))

		start = time.Now()
		for _, blk := range executedBlocks {
			retrievedBlk, err := indexer.GetBlockByHeight(blk.Block.Hght)
			if err != nil {
				panic(err)
			}
			if retrievedBlk.BlockID != blk.BlockID {
				panic("block id mismatch")
			}
		}
		fmt.Printf("Retrieved %s blocks by height in %s\n", formatNumberWithSuffix(blockCount), time.Since(start))

		start = time.Now()
		for _, blk := range executedBlocks {
			retrievedBlk, err := indexer.GetBlock(blk.BlockID)
			if err != nil {
				panic(err)
			}
			if retrievedBlk.BlockID != blk.BlockID {
				panic("block id mismatch")
			}
		}
		fmt.Printf("Retrieved %s blocks by id in %s\n", formatNumberWithSuffix(blockCount), time.Since(start))

		start = time.Now()
		for _, blk := range executedBlocks {
			txs := blk.Block.Txs
			for _, tx := range txs {
				retrievedTx, _, err := indexer.GetTransaction(tx.ID())
				if err != nil {
					panic(err)
				}
				if retrievedTx.ID() != tx.ID() {
					panic("transaction id mismatch")
				}
			}
		}
		fmt.Printf("Retrieved %s transactions by id in %s\n", formatNumberWithSuffix(txPerBlock*blockCount), time.Since(start))

		fmt.Printf("Database size: %s\n", must(getHumanReadableDirSize(tempDir)))
	}
}

func GenerateExecutedBlocks(
	parser *chaintest.Parser,
	parentID ids.ID,
	parentHeight uint64,
	parentTimestamp int64,
	timestampOffset int64,
	numBlocks int,
	txPerBlock int,
) []*chain.ExecutedBlock {
	executedBlocks := make([]*chain.ExecutedBlock, numBlocks)

	transactions := make([]*chain.Transaction, txPerBlock)
	for i := range transactions {
		transactions[i] = generateTestTx(parser)
	}

	for i := range executedBlocks {
		statelessBlock := &chain.StatelessBlock{
			Prnt:   parentID,
			Tmstmp: parentTimestamp + timestampOffset*int64(i),
			Hght:   parentHeight + 1 + uint64(i),
			Txs:    []*chain.Transaction{},
		}
		blkID := must(statelessBlock.ID())
		parentID = blkID

		executedBlocks[i] = must(chain.NewExecutedBlock(
			statelessBlock,
			[]*chain.Result{},
			fees.Dimensions{},
		))
	}
	return executedBlocks
}

func getHumanReadableDirSize(path string) (string, error) {
	cmd := exec.Command("du", "-sh", path)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.Split(string(output), "\t")[0], nil
}

func generateTestTx(parser *chaintest.Parser) *chain.Transaction {
	privKey := must(ed25519.GeneratePrivateKey())
	factory := auth.NewED25519Factory(privKey)

	tx := chain.NewTx(
		&chain.Base{
			Timestamp: 1234567000,
			ChainID:   ids.GenerateTestID(),
			MaxFee:    1234567,
		},

		[]chain.Action{
			&actions.Transfer{
				To:    codec.CreateAddress(0, ids.GenerateTestID()),
				Value: 12345,
				Memo:  []byte("test"),
			},
		},
	)

	return must(tx.Sign(factory, parser.ActionRegistry(), parser.AuthRegistry()))
}

func formatNumberWithSuffix(number float64) string {
	switch {
	case number >= 1_000_000_000_000:
		return fmt.Sprintf("%dt", int64(number/1_000_000_000_000))
	case number >= 1_000_000_000:
		return fmt.Sprintf("%db", int64(number/1_000_000_000))
	case number >= 1_000_000:
		return fmt.Sprintf("%dm", int64(number/1_000_000))
	case number >= 1_000:
		return fmt.Sprintf("%dk", int64(number/1_000))
	default:
		return fmt.Sprintf("%d", int64(number))
	}
}

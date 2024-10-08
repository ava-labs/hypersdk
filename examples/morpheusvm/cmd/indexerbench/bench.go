package main

import (
	"fmt"
	"math/rand/v2"
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
	parser := chaintest.NewEmptyParser()
	(*parser.ActionRegistry()).Register(&actions.Transfer{}, nil)
	(*parser.OutputRegistry()).Register(&actions.TransferResult{}, nil)
	(*parser.AuthRegistry()).Register(&auth.ED25519{}, nil)

	indexer := must(indexer.NewIndexer(PARENT_TEST_DIR, parser))

	lastBlocks, err := indexer.GetLatestBlock()
	if err != nil {
		panic(err)
	}

	lastBlockHeight := lastBlocks.Block.Hght

	for i := 0; i < 1000; i++ {
		const blockCount = 1000
		const txPerBlock = 1000

		start := time.Now()
		executedBlocks := GenerateExecutedBlocks(parser, ids.Empty, lastBlockHeight, 0, 1, blockCount, txPerBlock)
		lastBlockHeight += uint64(blockCount)

		// Sequentially accept blocks
		for _, blk := range executedBlocks {
			if err := indexer.Accept(blk); err != nil {
				panic(err)
			}
		}

		fmt.Printf("Processed %s blocks with %s transactions in %s\n", formatNumberWithSuffix(blockCount), formatNumberWithSuffix(txPerBlock*blockCount), time.Since(start))

		start = time.Now()
		randomHeights := make([]uint64, blockCount)
		for i := range randomHeights {
			randomHeights[i] = uint64(rand.IntN(int(lastBlockHeight + 1)))
		}

		blockIDs := make([]ids.ID, 0, blockCount)

		// Retrieve and check random blocks by height
		for _, height := range randomHeights {
			retrievedBlk, err := indexer.GetBlockByHeight(height)
			if err != nil {
				panic(err)
			}
			// Store block ID for later use
			blockIDs = append(blockIDs, retrievedBlk.BlockID)
		}
		fmt.Printf("Retrieved %s random blocks by height in %s\n", formatNumberWithSuffix(blockCount), time.Since(start))

		// Retrieve and check the same blocks by ID
		start = time.Now()
		for _, blockID := range blockIDs {
			retrievedBlk, err := indexer.GetBlock(blockID)
			if err != nil {
				panic(err)
			}
			if retrievedBlk.BlockID != blockID {
				panic("block id mismatch")
			}
		}
		fmt.Printf("Retrieved %s blocks by id in %s\n", formatNumberWithSuffix(blockCount), time.Since(start))

		// Retrieve and check a random transaction from each block
		start = time.Now()
		for _, blockID := range blockIDs {
			retrievedBlk, err := indexer.GetBlock(blockID)
			if err != nil {
				panic(err)
			}
			if len(retrievedBlk.Block.Txs) > 0 {
				randomTxIndex := rand.IntN(len(retrievedBlk.Block.Txs))
				randomTx := retrievedBlk.Block.Txs[randomTxIndex]
				retrievedTx, _, err := indexer.GetTransaction(randomTx.ID())
				if err != nil {
					panic(err)
				}
				if retrievedTx.Auth.Actor() != randomTx.Auth.Actor() {
					panic("transaction id mismatch")
				}
			}
		}
		fmt.Printf("Retrieved and checked %s random transactions in %s\n", formatNumberWithSuffix(blockCount), time.Since(start))

		fmt.Printf("Database size: %s\n", must(getHumanReadableDirSize(PARENT_TEST_DIR)))
		fmt.Printf("Last block height: %d\n", lastBlockHeight)
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

	ts := int64(1234567+rand.IntN(100)) * 1000

	tx := chain.NewTx(
		&chain.Base{
			Timestamp: ts,
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

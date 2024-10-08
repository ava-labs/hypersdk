package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
)

const TEST_DIR = "/var/tmp/indexer-bench"

func main() {
	//clean up dir
	err := os.RemoveAll(TEST_DIR)
	if err != nil {
		log.Fatalf("removing test dir: %s", err)
	}
	log.Println("creating test dir")
	err = os.MkdirAll(TEST_DIR, 0o755)
	if err != nil {
		log.Fatalf("creating test dir: %s", err)
	}

	parser := chaintest.NewEmptyParser()
	(*parser.ActionRegistry()).Register(&actions.Transfer{}, nil)
	(*parser.OutputRegistry()).Register(&actions.TransferResult{}, nil)
	(*parser.AuthRegistry()).Register(&auth.ED25519{}, nil)

	indexer, err := indexer.NewIndexer(TEST_DIR, parser)
	if err != nil {
		log.Fatalf("creating indexer: %s", err)
	}

	const blockCount = 1000
	const txPerBlock = 1_000
	const blocksPerCycle = 1_000

	err = fillIndexerAsync(indexer, parser, blockCount, txPerBlock, blocksPerCycle)
	if err != nil {
		log.Fatalf("filling indexer: %s", err)
	}

	err = benchBlockByHeight(indexer, 1_000, blockCount-1)
	if err != nil {
		log.Fatalf("benchmarking block by id: %s", err)
	}

	log.Println("done")
}

func formatNumberWithSuffix(number int) string {
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

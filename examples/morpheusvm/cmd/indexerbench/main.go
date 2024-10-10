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

const CLEANUP_ON_START = true

func main() {
	var err error
	if CLEANUP_ON_START {
		err := os.RemoveAll(TEST_DIR)
		if err != nil {
			log.Fatalf("removin g test dir: %s", err)
		}
		fmt.Println(" - - - - - - ")
	}

	err = os.MkdirAll(TEST_DIR, 0o755)
	if err != nil {
		log.Fatalf("creating test dir: %s", err)
	}

	parser := chaintest.NewEmptyParser()
	(*parser.ActionCodec()).Register(&actions.Transfer{}, nil)
	(*parser.OutputCodec()).Register(&actions.TransferResult{}, nil)
	(*parser.AuthCodec()).Register(&auth.ED25519{}, nil)

	const txPerBlock = 10000
	const blockCount = 100_000 / txPerBlock
	const itemsToBenchmark = 100
	const blockWindow = 10_000_000

	idxer, err := indexer.NewIndexer(TEST_DIR, parser, blockWindow)
	if err != nil {
		log.Fatalf("creating indexer: %s", err)
	}

	for i := 0; i < 1000; i++ {
		err = fillIndexerAsync(idxer, parser, blockCount, txPerBlock)
		if err != nil {
			log.Fatalf("filling indexer: %s", err)
		}

		err = bench(idxer, itemsToBenchmark, blockWindow)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Println("done")
}

func bench(idxer *indexer.Indexer, itemsToBenchmark int, blockWindow int) error {
	if err := benchBlocksById(idxer, itemsToBenchmark, blockWindow); err != nil {
		return fmt.Errorf("benchmarking blocks by id: %w", err)
	}

	if err := benchBlockByHeight(idxer, itemsToBenchmark, blockWindow); err != nil {
		return fmt.Errorf("benchmarking block by height: %w", err)
	}

	if err := benchTransactionsByID(idxer, itemsToBenchmark, blockWindow); err != nil {
		return fmt.Errorf("benchmarking transactions by id: %w", err)
	}

	return nil
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

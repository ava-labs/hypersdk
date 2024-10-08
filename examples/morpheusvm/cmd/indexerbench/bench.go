package main

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
)

func benchBlockByHeight(idxer *indexer.Indexer, blocksToRetrieve int, maxHeight int) error {
	randomHeights := make([]uint64, blocksToRetrieve)
	for i := 0; i < blocksToRetrieve; i++ {
		randomHeights[i] = uint64(rand.IntN(maxHeight))
	}

	blocks := make([]*chain.ExecutedBlock, blocksToRetrieve)

	start := time.Now()
	errChan := make(chan error, len(randomHeights))
	var wg sync.WaitGroup
	for i, height := range randomHeights {
		wg.Add(1)
		go func(i int, height uint64) {
			defer wg.Done()
			block, err := idxer.GetBlockByHeight(height)
			if err != nil {
				errChan <- fmt.Errorf("getting block by height %d: %w", height, err)
				return
			}
			blocks[i] = block
		}(i, height)
	}
	wg.Wait()
	close(errChan)
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	elapsed := time.Since(start)

	rps := float64(blocksToRetrieve) / elapsed.Seconds()
	fmt.Printf("Retrieved %d blocks by height in %s (%.2f RPS)\n", blocksToRetrieve, elapsed, rps)

	for i, block := range blocks {
		if block.Block.Hght != randomHeights[i] {
			return fmt.Errorf("block height mismatch: %d != %d", block.Block.Hght, randomHeights[i])
		}

		// all actions have to be transfers with value equal to the block height
		anyAction := block.Block.Txs[0].Actions[0].(*actions.Transfer)
		if anyAction.Value != block.Block.Hght {
			return fmt.Errorf("first transaction amount mismatch: %d", anyAction.Value)
		}
	}

	return nil
}

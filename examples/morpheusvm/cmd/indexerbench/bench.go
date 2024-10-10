package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
)

func benchBlockByHeight(idxer *indexer.Indexer, blocksToRetrieve int, blockWindow int) error {
	start := time.Now()
	blocks, err := retrieveRandomBlocks(idxer, blocksToRetrieve, blockWindow)
	if err != nil {
		return fmt.Errorf("retrieving blocks by height: %w", err)
	}

	elapsed := time.Since(start)
	rps := float64(blocksToRetrieve) / elapsed.Seconds()
	log.Printf("Retrieved %d blocks by height in %s (%.2f RPS)\n", blocksToRetrieve, elapsed, rps)

	//check if it all looks sane
	for _, block := range blocks {
		// all actions have to be transfers with value equal to the block height
		anyAction := block.Block.Txs[0].Actions[0].(*actions.Transfer)
		if anyAction.Value != block.Block.Hght {
			return fmt.Errorf("first transaction amount mismatch: %d", anyAction.Value)
		}
		if anyAction.Value == 0 {
			return fmt.Errorf("first transaction amount is 0")
		}
	}

	return nil
}

func benchTransactionsByID(idxer *indexer.Indexer, transactionsToRetrieve int, blockWindow int) error {
	randomBlocks, err := retrieveRandomBlocks(idxer, transactionsToRetrieve, blockWindow)
	if err != nil {
		return fmt.Errorf("retrieving blocks by height: %w", err)
	}

	randomIds := make([]ids.ID, transactionsToRetrieve)
	for i, block := range randomBlocks {
		txLen := len(block.Block.Txs)
		randomTx := block.Block.Txs[rand.IntN(txLen)]
		randomIds[i] = randomTx.ID()
	}

	start := time.Now()
	txResponses := make([]*indexer.GetTxResponse, transactionsToRetrieve)
	for i, id := range randomIds {
		tx, txResult, err := idxer.GetTransaction(id)
		if err != nil {
			return fmt.Errorf("getting transaction by ID %s: %w", id, err)
		}
		txResponses[i] = &indexer.GetTxResponse{
			Transaction: tx,
			Result:      txResult,
		}
	}

	elapsed := time.Since(start)
	rps := float64(transactionsToRetrieve) / elapsed.Seconds()
	log.Printf("Retrieved %d transactions by ID in %s (%.2f RPS)\n", transactionsToRetrieve, elapsed, rps)

	for i, randomBlock := range randomBlocks {
		// all actions have to be transfers with value equal to the block height
		if txResponses[i].Transaction.Actions[0].(*actions.Transfer).Value != randomBlock.Block.Hght {
			return fmt.Errorf("transaction amount mismatch: expected %d, got %d", randomBlock.Block.Hght, txResponses[i].Transaction.Actions[0].(*actions.Transfer).Value)
		}
	}

	return nil
}

func benchBlocksById(idxer *indexer.Indexer, blocksToRetrieve int, blockWindow int) error {
	randomBlocks, err := retrieveRandomBlocks(idxer, blocksToRetrieve, blockWindow)
	if err != nil {
		return fmt.Errorf("retrieving blocks by height: %w", err)
	}

	randomIds := make([]ids.ID, blocksToRetrieve)
	for i, block := range randomBlocks {
		randomIds[i] = block.BlockID
	}

	start := time.Now()
	blocks := make([]*chain.ExecutedBlock, blocksToRetrieve)
	errChan := make(chan error, blocksToRetrieve)
	var wg sync.WaitGroup
	for i, id := range randomIds {
		wg.Add(1)
		go func(i int, id ids.ID) {
			defer wg.Done()
			block, err := idxer.GetBlock(id)
			if err != nil {
				errChan <- fmt.Errorf("getting block by ID %s: %w", id, err)
				return
			}
			blocks[i] = block
		}(i, id)
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
	log.Printf("Retrieved %d blocks by ID in %s (%.2f RPS)\n", blocksToRetrieve, elapsed, rps)

	for i, randomBlock := range randomBlocks {
		if randomBlock.BlockID != blocks[i].BlockID {
			return fmt.Errorf("block ID mismatch: expected %s, got %s", randomBlock.BlockID, blocks[i].BlockID)
		}
		if randomBlock.Block.Hght != blocks[i].Block.Hght {
			return fmt.Errorf("block height mismatch: expected %d, got %d", randomBlock.Block.Hght, blocks[i].Block.Hght)
		}
		if blocks[i].BlockID == ids.Empty {
			return fmt.Errorf("block ID is empty")
		}
	}

	return nil
}

func retrieveRandomBlocks(idxer *indexer.Indexer, blocksToRetrieve int, blockWindow int) ([]*chain.ExecutedBlock, error) {
	latestBlock, err := idxer.GetLatestBlock()
	if err != nil {
		return nil, fmt.Errorf("getting latest block: %w", err)
	}

	maxHeight := int(latestBlock.Block.Hght)

	randomHeights := make([]uint64, blocksToRetrieve)
	for i := 0; i < blocksToRetrieve; i++ {
		minHeight := max(1, int(maxHeight-(blockWindow)+1))
		randomHeights[i] = uint64(rand.IntN(maxHeight-minHeight) + minHeight)
	}

	blocks := make([]*chain.ExecutedBlock, blocksToRetrieve)

	errChan := make(chan error, blocksToRetrieve)
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
			return nil, err
		}
	}
	return blocks, nil
}

package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/fees"
)

func fillIndexerAsync(idxer *indexer.Indexer, parser *chaintest.Parser, blockCount int, txPerBlock int, blocksPerCycle int) error {
	parentID := ids.Empty
	parentHeight := int64(0)
	parentTimestamp := int64(1234567000)

	for remainingBlocks := blockCount; remainingBlocks > 0; {
		cycleBlocks := blocksPerCycle
		if remainingBlocks < cycleBlocks {
			cycleBlocks = remainingBlocks
		}

		executedBlocks, err := generateExecutedBlocks(generateExecutedBlocksParams{
			Parser:          parser,
			ParentID:        parentID,
			ParentHeight:    parentHeight,
			ParentTimestamp: parentTimestamp,
			TimestampOffset: 1,
			NumBlocks:       cycleBlocks,
			TxPerBlock:      txPerBlock,
		})
		if err != nil {
			return fmt.Errorf("generating executed blocks: %w", err)
		}

		start := time.Now()
		errChan := make(chan error, len(executedBlocks))
		for _, blk := range executedBlocks {
			go func(b *chain.ExecutedBlock) {
				errChan <- idxer.Accept(b)
			}(blk)
		}

		for range executedBlocks {
			if err := <-errChan; err != nil {
				return fmt.Errorf("accepting block: %w", err)
			}
		}

		lastBlock := executedBlocks[len(executedBlocks)-1]
		parentID = lastBlock.BlockID
		parentHeight = int64(lastBlock.Block.Hght)
		parentTimestamp = lastBlock.Block.Tmstmp

		elapsed := time.Since(start)
		dirSize, err := getHumanReadableDirSize(TEST_DIR)
		if err != nil {
			return fmt.Errorf("getting directory size: %w", err)
		}
		log.Printf("accepted %s blocks containing %s txs in %s. Database occupies %s on disk with %s blocks total\n",
			formatNumberWithSuffix(cycleBlocks),
			formatNumberWithSuffix(cycleBlocks*txPerBlock),
			elapsed,
			dirSize,
			formatNumberWithSuffix(int(parentHeight)+1),
		)

		remainingBlocks -= cycleBlocks
	}

	return nil
}

type generateExecutedBlocksParams struct {
	Parser          *chaintest.Parser
	ParentID        ids.ID
	ParentHeight    int64
	ParentTimestamp int64
	TimestampOffset int64
	NumBlocks       int
	TxPerBlock      int
}

func generateExecutedBlocks(params generateExecutedBlocksParams) ([]*chain.ExecutedBlock, error) {
	executedBlocks := make([]*chain.ExecutedBlock, params.NumBlocks)

	for i := range executedBlocks {
		statelessBlock := &chain.StatelessBlock{
			Prnt:   params.ParentID,
			Tmstmp: params.ParentTimestamp + params.TimestampOffset*int64(i*1000),
			Hght:   uint64(params.ParentHeight) + 1 + uint64(i),
			Txs:    make([]*chain.Transaction, params.TxPerBlock),
		}

		sampleTx, err := generateTestTx(params.Parser, statelessBlock.Hght, statelessBlock.Tmstmp)
		if err != nil {
			return nil, fmt.Errorf("generating test transaction: %w", err)
		}
		for j := range statelessBlock.Txs {
			statelessBlock.Txs[j] = sampleTx
		}

		blkID, err := statelessBlock.ID()
		if err != nil {
			return nil, fmt.Errorf("getting block ID: %w", err)
		}
		params.ParentID = blkID

		executedBlock, err := chain.NewExecutedBlock(
			statelessBlock,
			make([]*chain.Result, params.TxPerBlock),
			fees.Dimensions{},
		)
		if err != nil {
			return nil, fmt.Errorf("creating executed block: %w", err)
		}

		resultPacker := &wrappers.Packer{MaxSize: consts.NetworkSizeLimit}
		err = codec.LinearCodec.MarshalInto(actions.TransferResult{
			ReceiverBalance: uint64(statelessBlock.Hght),
			SenderBalance:   uint64(statelessBlock.Hght),
		}, resultPacker)
		if err != nil {
			return nil, fmt.Errorf("marshalling sample result: %w", err)
		}

		for j := range statelessBlock.Txs {
			executedBlock.Results[j] = &chain.Result{
				Success: true,
				Error:   []byte{},
				Outputs: [][]byte{resultPacker.Bytes},
				Units:   [5]uint64{0, 0, 0, 0, 0},
				Fee:     uint64(statelessBlock.Hght),
			}
		}

		executedBlocks[i] = executedBlock
	}
	return executedBlocks, nil
}

func generateTestTx(parser *chaintest.Parser, blockHeight uint64, timestamp int64) (*chain.Transaction, error) {
	privKey, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generating private key: %w", err)
	}
	factory := auth.NewED25519Factory(privKey)

	tx := chain.NewTx(
		&chain.Base{
			Timestamp: timestamp,
			ChainID:   ids.GenerateTestID(),
			MaxFee:    1234567,
		},

		[]chain.Action{
			&actions.Transfer{
				To:    codec.CreateAddress(0, ids.GenerateTestID()),
				Value: blockHeight,
				Memo:  []byte("hello"),
			},
		},
	)

	signedTx, err := tx.Sign(factory, parser.ActionRegistry(), parser.AuthRegistry())
	if err != nil {
		return nil, fmt.Errorf("signing transaction: %w", err)
	}
	return signedTx, nil
}
func getHumanReadableDirSize(path string) (string, error) {
	var output []byte
	var err error
	for i := 0; i < 3; i++ {
		cmd := exec.Command("du", "-sh", path)
		output, err = cmd.Output()
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return "", fmt.Errorf("executing du command: %w", err)
	}
	return strings.Split(string(output), "\t")[0], nil
}

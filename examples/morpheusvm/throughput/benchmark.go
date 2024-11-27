// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package throughput implements the Benchmark interface. This package is not
// required to be implemented by the VM developer.

package throughput

import (
	"encoding/json"
	"sync"

	"github.com/ava-labs/avalanchego/utils/math"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/abi/dynamic"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/throughput"
	"github.com/ava-labs/hypersdk/utils"
)

var _ throughput.Benchmark = &Benchmark{}

type Benchmark struct {
	l   sync.Mutex
	abi abi.ABI

	// Specific to MorpheusVM
	tradingVolume uint64
}

func NewBenchmark() (*Benchmark, error) {
	abi, err := abi.NewABI(vm.ActionParser.GetRegisteredTypes(), vm.OutputParser.GetRegisteredTypes())
	if err != nil {
		return nil, err
	}
	return &Benchmark{abi: abi}, nil
}

func (b *Benchmark) LogOutputs(outputs [][]byte) error {
	for _, output := range outputs {
		resultJSON, err := dynamic.UnmarshalOutput(b.abi, output)
		if err != nil {
			return err
		}

		var result actions.TransferResult
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			return err
		}

		amount := math.AbsDiff(result.ReceiverBalance, result.SenderBalance)
		b.l.Lock()
		b.tradingVolume += amount
		b.l.Unlock()
	}
	return nil
}

func (b *Benchmark) LogState() {
	b.l.Lock()
	utils.Outf("{{green}}Trading Volume per Second:{{/}} %d\n", b.tradingVolume)
	b.tradingVolume = 0
	b.l.Unlock()
}

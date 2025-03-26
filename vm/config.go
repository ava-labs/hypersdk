// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chainindex"

	hcontext "github.com/ava-labs/hypersdk/context"
)

const (
	VMNamespaceKey = "vm"
)

type Config struct {
	MempoolSize                      int           `json:"mempoolSize"`
	MempoolSponsorSize               int           `json:"mempoolSponsorSize"`
	AuthVerificationCores            int           `json:"authVerificationCores"`
	RootGenerationCores              int           `json:"rootGenerationCores"`
	StateHistoryLength               int           `json:"stateHistoryLength"`               // how many roots back of data to keep to serve state queries
	IntermediateNodeCacheSize        int           `json:"intermediateNodeCacheSize"`        // how many bytes to keep in intermediate cache
	StateIntermediateWriteBufferSize int           `json:"stateIntermediateWriteBufferSize"` // how many bytes to keep unwritten in intermediate cache
	StateIntermediateWriteBatchSize  int           `json:"stateIntermediateWriteBatchSize"`  // how many bytes to write from intermediate cache at once
	ValueNodeCacheSize               int           `json:"valueNodeCacheSize"`               // how many bytes to keep in value cache
	TargetGossipDuration             time.Duration `json:"targetGossipDuration"`
}

func NewConfig() Config {
	return Config{
		MempoolSize:                      2_048,
		AuthVerificationCores:            1,
		RootGenerationCores:              1,
		MempoolSponsorSize:               32,
		StateHistoryLength:               256,
		IntermediateNodeCacheSize:        4 * units.GiB,
		StateIntermediateWriteBufferSize: 32 * units.MiB,
		StateIntermediateWriteBatchSize:  4 * units.MiB,
		ValueNodeCacheSize:               2 * units.GiB,
		TargetGossipDuration:             20 * time.Millisecond,
	}
}

func GetVMConfig(config hcontext.Config) (Config, error) {
	return hcontext.GetConfig(config, VMNamespaceKey, NewConfig())
}

func GetChainConfig(config hcontext.Config) (chain.Config, error) {
	return hcontext.GetConfig(config, chainNamespace, chain.NewDefaultConfig())
}

func GetChainIndexConfig(config hcontext.Config) (chainindex.Config, error) {
	return hcontext.GetConfig(config, chainIndexNamespace, chainindex.NewDefaultConfig())
}

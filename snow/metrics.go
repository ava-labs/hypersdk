// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	blockBuild   metric.Averager
	blockParse   metric.Averager
	blockVerify  metric.Averager
	blockAccept  metric.Averager
	blockProcess metric.Averager
}

func newMetrics(r *prometheus.Registry) (*Metrics, error) {
	blockBuild, err := metric.NewAverager(
		"chain_block_build",
		"time spent building blocks",
		r,
	)
	if err != nil {
		return nil, err
	}
	blockParse, err := metric.NewAverager(
		"chain_block_parse",
		"time spent parsing blocks",
		r,
	)
	if err != nil {
		return nil, err
	}
	blockVerify, err := metric.NewAverager(
		"chain_block_verify",
		"time spent verifying blocks",
		r,
	)
	if err != nil {
		return nil, err
	}
	blockAccept, err := metric.NewAverager(
		"chain_block_accept",
		"time spent accepting blocks",
		r,
	)
	if err != nil {
		return nil, err
	}
	blockProcess, err := metric.NewAverager(
		"chain_block_process",
		"time spent processing blocks",
		r,
	)
	if err != nil {
		return nil, err
	}

	m := &Metrics{
		blockBuild:   blockBuild,
		blockParse:   blockParse,
		blockVerify:  blockVerify,
		blockAccept:  blockAccept,
		blockProcess: blockProcess,
	}

	return m, nil
}

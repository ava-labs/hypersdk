// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
)

type metrics struct {
	transfer prometheus.Counter
}

func newMetrics(gatherer ametrics.MultiGatherer) (*metrics, error) {
	m := &metrics{
		transfer: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "actions",
			Name:      "transfer",
			Help:      "number of transfer actions",
		}),
	}
	r := prometheus.NewRegistry()
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.transfer),

		gatherer.Register(consts.Name, r),
	)
	return m, errs.Err
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	mints     prometheus.Counter
	transfers prometheus.Counter
}

func newMetrics(gatherer ametrics.MultiGatherer) (*metrics, error) {
	m := &metrics{
		mints: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "actions",
			Name:      "mints",
			Help:      "number of mint actions",
		}),
		transfers: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "actions",
			Name:      "transfers",
			Help:      "number of transfer actions",
		}),
	}
	r := prometheus.NewRegistry()
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.mints),
		r.Register(m.transfers),
		gatherer.Register("transfervm", r),
	)
	return m, errs.Err
}

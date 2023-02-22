// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	itemsIndexed   prometheus.Counter
	itemsUnindexed prometheus.Counter
}

func newMetrics(gatherer ametrics.MultiGatherer) (*metrics, error) {
	m := &metrics{
		itemsIndexed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "actions",
			Name:      "items_indexed",
			Help:      "number of items indexed in state",
		}),
		itemsUnindexed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "actions",
			Name:      "items_unindexed",
			Help:      "number of items unindexed from state",
		}),
	}
	r := prometheus.NewRegistry()
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.itemsIndexed),
		r.Register(m.itemsUnindexed),
		gatherer.Register("index_vm", r),
	)
	return m, errs.Err
}

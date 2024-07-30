// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/vm/event"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
)

var _ event.SubscriptionFactory[*chain.StatelessBlock] = (*MetricsFactory)(nil)

func NewMetricsFactory(metrics ametrics.MultiGatherer) *MetricsFactory {
	return &MetricsFactory{
		metrics: metrics,
	}
}

type MetricsFactory struct {
	metrics ametrics.MultiGatherer
}

func (m *MetricsFactory) New() (event.Subscription[*chain.StatelessBlock], error) {
	return newMetrics(m.metrics)
}

type metrics struct {
	transfer prometheus.Counter
}

func (m metrics) Accept(blk *chain.StatelessBlock) error {
	for _, tx := range blk.Txs {
		for _, action := range tx.Actions {
			switch action.(type) { //nolint:gocritic
			case *actions.Transfer:
				m.transfer.Inc()
			}
		}
	}

	return nil
}

func (m metrics) Close() error {
	return nil
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

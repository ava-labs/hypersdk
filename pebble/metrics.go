// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pebble

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/cockroachdb/pebble"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	delayStart time.Time
	writeStall metric.Averager

	getLatency metric.Averager

	l0Compactions     prometheus.Counter
	otherCompactions  prometheus.Counter
	activeCompactions prometheus.Gauge
}

func newMetrics() (*prometheus.Registry, *metrics, error) {
	r := prometheus.NewRegistry()
	writeStall, err := metric.NewAverager(
		"pebble",
		"write_stall",
		"time spent waiting for disk write",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	getLatency, err := metric.NewAverager(
		"pebble",
		"read_latency",
		"time spent waiting for db get",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	m := &metrics{
		writeStall: writeStall,
		getLatency: getLatency,
		l0Compactions: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "pebble",
			Name:      "l0_compactions",
			Help:      "number of l0 compactions",
		}),
		otherCompactions: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "pebble",
			Name:      "other_compactions",
			Help:      "number of l1+ compactions",
		}),
		activeCompactions: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "pebble",
			Name:      "active_compactions",
			Help:      "number of active compactions",
		}),
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.l0Compactions),
		r.Register(m.otherCompactions),
		r.Register(m.activeCompactions),
	)
	return r, m, errs.Err
}

func (d *Database) onCompactionBegin(info pebble.CompactionInfo) {
	d.metrics.activeCompactions.Inc()
	l0 := info.Input[0]
	if l0.Level == 0 {
		d.metrics.l0Compactions.Inc()
	} else {
		d.metrics.otherCompactions.Inc()
	}
}

func (d *Database) onCompactionEnd(pebble.CompactionInfo) {
	d.metrics.activeCompactions.Dec()
}

func (d *Database) onWriteStallBegin(pebble.WriteStallBeginInfo) {
	d.metrics.delayStart = time.Now()
}

func (d *Database) onWriteStallEnd() {
	d.metrics.writeStall.Observe(float64(time.Since(d.metrics.delayStart)))
}

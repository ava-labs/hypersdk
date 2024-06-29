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

const metricsInterval = 10 * time.Second

type metrics struct {
	delayStart time.Time
	writeStall metric.Averager

	getLatency metric.Averager

	l0Compactions     prometheus.Counter
	otherCompactions  prometheus.Counter
	activeCompactions prometheus.Gauge

	tombstoneCount     prometheus.Gauge
	obsoleteTableSize  prometheus.Gauge
	obsoleteTableCount prometheus.Gauge
	zombieTableSize    prometheus.Gauge
	zombieTableCount   prometheus.Gauge
	obsoleteWALSize    prometheus.Gauge
	obsoleteWALCount   prometheus.Gauge
}

func newMetrics() (*prometheus.Registry, *metrics, error) {
	r := prometheus.NewRegistry()
	writeStall, err := metric.NewAverager(
		"pebble_write_stall",
		"time spent waiting for disk write",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	getLatency, err := metric.NewAverager(
		"pebble_read_latency",
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
		tombstoneCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "pebble",
			Name:      "tombstone_count",
			Help:      "approximate count of internal tombstones",
		}),
		obsoleteTableSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "pebble",
			Name:      "obsolete_table_size",
			Help:      "number of bytes present in tables no longer referenced by the db",
		}),
		obsoleteTableCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "pebble",
			Name:      "obsolete_table_count",
			Help:      "number of table files no longer referenced by the db",
		}),
		zombieTableSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "pebble",
			Name:      "zombie_table_size",
			Help:      "number of bytes present in tables no longer referenced by the db that are referenced by iterators",
		}),
		zombieTableCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "pebble",
			Name:      "zombie_table_count",
			Help:      "number of table files no longer referenced by the db that are referenced by iterators",
		}),
		obsoleteWALSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "pebble",
			Name:      "obsolete_wal_size",
			Help:      "number of bytes present in WAL no longer needed by the db",
		}),
		obsoleteWALCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "pebble",
			Name:      "obsolete_wal_count",
			Help:      "number of WAL files no longer needed by the db",
		}),
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.l0Compactions),
		r.Register(m.otherCompactions),
		r.Register(m.activeCompactions),
		r.Register(m.tombstoneCount),
		r.Register(m.obsoleteTableSize),
		r.Register(m.obsoleteTableCount),
		r.Register(m.zombieTableSize),
		r.Register(m.zombieTableCount),
		r.Register(m.obsoleteWALSize),
		r.Register(m.obsoleteWALCount),
	)
	return r, m, errs.Err
}

func (db *Database) onCompactionBegin(info pebble.CompactionInfo) {
	db.metrics.activeCompactions.Inc()
	l0 := info.Input[0]
	if l0.Level == 0 {
		db.metrics.l0Compactions.Inc()
	} else {
		db.metrics.otherCompactions.Inc()
	}
}

func (db *Database) onCompactionEnd(pebble.CompactionInfo) {
	db.metrics.activeCompactions.Dec()
}

func (db *Database) onWriteStallBegin(pebble.WriteStallBeginInfo) {
	db.metrics.delayStart = time.Now()
}

func (db *Database) onWriteStallEnd() {
	db.metrics.writeStall.Observe(float64(time.Since(db.metrics.delayStart)))
}

func (db *Database) collectMetrics() {
	t := time.NewTicker(metricsInterval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			metrics := db.db.Metrics()
			db.metrics.tombstoneCount.Set(float64(metrics.Keys.TombstoneCount))
			db.metrics.obsoleteTableSize.Set(float64(metrics.Table.ObsoleteSize))
			db.metrics.obsoleteTableCount.Set(float64(metrics.Table.ObsoleteCount))
			db.metrics.zombieTableSize.Set(float64(metrics.Table.ZombieSize))
			db.metrics.zombieTableCount.Set(float64(metrics.Table.ZombieCount))
			db.metrics.obsoleteWALSize.Set(float64(metrics.WAL.ObsoletePhysicalSize))
			db.metrics.obsoleteWALCount.Set(float64(metrics.WAL.ObsoleteFiles))
		case <-db.closing:
			return
		}
	}
}

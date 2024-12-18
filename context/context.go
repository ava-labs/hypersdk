// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package context

import (
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics interface {
	// Register adds the outputs of [gatherer] to the results of future calls to
	// Gather with the provided [name] added to the metrics.
	Register(name string, gatherer prometheus.Gatherer) error
}

type Context struct {
	log    logging.Logger
	tracer trace.Tracer

	config  Config
	metrics Metrics
}

func New(
	log logging.Logger,
	metrics Metrics,
	configBytes []byte,
) (*Context, error) {
	config, err := NewConfig(configBytes)
	if err != nil {
		return nil, err
	}
	traceConfig, err := GetConfig(config, "tracer", trace.Config{Enabled: false})
	if err != nil {
		return nil, err
	}
	tracer, err := trace.New(traceConfig)
	if err != nil {
		return nil, err
	}
	return &Context{
		log:     log,
		tracer:  tracer,
		metrics: metrics,
		config:  config,
	}, nil
}

func (c *Context) MakeRegistry(name string) (*prometheus.Registry, error) {
	registry := prometheus.NewRegistry()
	if err := c.metrics.Register(name, registry); err != nil {
		return nil, err
	}
	return registry, nil
}

func (c *Context) Log() logging.Logger {
	return c.log
}

func (c *Context) Tracer() trace.Tracer {
	return c.tracer
}

func GetConfigFromContext[T any](ctx *Context, key string, defaultConfig T) (T, error) {
	return GetConfig(ctx.config, key, defaultConfig)
}

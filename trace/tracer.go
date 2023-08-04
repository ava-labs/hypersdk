// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package trace

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/ava-labs/avalanchego/trace"

	"github.com/uptrace/uptrace-go/uptrace"
)

const (
	tracerExportTimeout = 10 * time.Second
	// [tracerProviderShutdownTimeout] is longer than [tracerExportTimeout] so
	// in-flight exports can finish before the tracer provider shuts down.
	tracerProviderShutdownTimeout = 15 * time.Second
)

type Config struct {
	// Used to flag if tracing should be performed
	Enabled bool `json:"enabled"`

	// The fraction of traces to sample.
	// If >= 1 always samples.
	// If <= 0 never samples.
	TraceSampleRate float64 `json:"traceSampleRate"`

	AppName string `json:"appName"`
	Agent   string `json:"agent"`
	Version string `json:"version"`
	DSN     string `json:"dsn"`
}

type tracer struct {
	oteltrace.Tracer

	tp oteltrace.TracerProvider
}

func (*tracer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), tracerProviderShutdownTimeout)
	defer cancel()
	return uptrace.Shutdown(ctx)
}

func New(config *Config) (trace.Tracer, error) {
	if !config.Enabled {
		return &noOpTracer{
			t: oteltrace.NewNoopTracerProvider().Tracer(config.AppName),
		}, nil
	}

	uptrace.ConfigureOpentelemetry(
		uptrace.WithDSN(config.DSN),
		uptrace.WithServiceName(config.Agent),
		uptrace.WithServiceVersion(config.Version),
		uptrace.WithBatchSpanProcessorOption(
			sdktrace.WithExportTimeout(tracerExportTimeout),
		),
		uptrace.WithTraceSampler(sdktrace.TraceIDRatioBased(config.TraceSampleRate)),
	)
	tracerProvider := otel.GetTracerProvider()
	return &tracer{
		Tracer: tracerProvider.Tracer(config.AppName),
		tp:     tracerProvider,
	}, nil
}

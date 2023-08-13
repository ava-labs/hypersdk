// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package trace

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/encoding/gzip"

	"github.com/ava-labs/avalanchego/trace"
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

	AppName  string `json:"appName"`
	Agent    string `json:"agent"`
	Version  string `json:"version"`
	Endpoint string `json:"endpoint"`
	DSN      string `json:"dsn"`
	Insecure bool   `json:"insecure"`
}

type tracer struct {
	oteltrace.Tracer

	tp *sdktrace.TracerProvider
}

func (x *tracer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), tracerProviderShutdownTimeout)
	defer cancel()
	return x.tp.Shutdown(ctx)
}

func New(config *Config) (trace.Tracer, error) {
	if !config.Enabled {
		return &noOpTracer{
			t: oteltrace.NewNoopTracerProvider().Tracer(config.AppName),
		}, nil
	}

	exporterOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.Endpoint),
		otlptracegrpc.WithHeaders(map[string]string{
			"uptrace-dsn": config.DSN,
		}),
		otlptracegrpc.WithCompressor(gzip.Name),
	}
	if config.Insecure {
		exporterOpts = append(exporterOpts, otlptracegrpc.WithInsecure())
	}
	exporter, err := otlptracegrpc.New(
		context.Background(),
		exporterOpts...,
	)
	if err != nil {
		panic(err)
	}

	tracerProviderOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithBatcher(exporter, sdktrace.WithExportTimeout(tracerExportTimeout)),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("version", config.Version),
				semconv.ServiceNameKey.String(config.Agent),
			),
		),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.TraceSampleRate)),
	}
	tracerProvider := sdktrace.NewTracerProvider(tracerProviderOpts...)

	return &tracer{
		Tracer: tracerProvider.Tracer(config.AppName),
		tp:     tracerProvider,
	}, nil
}

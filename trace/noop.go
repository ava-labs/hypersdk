// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package trace

import (
	"context"

	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/ava-labs/avalanchego/trace"
)

var _ trace.Tracer = (*noOpTracer)(nil)

// noOpTracer is an implementation of trace.Tracer that does nothing.
type noOpTracer struct {
	t oteltrace.Tracer
}

func (n noOpTracer) Start(
	ctx context.Context,
	spanName string,
	opts ...oteltrace.SpanStartOption,
) (context.Context, oteltrace.Span) {
	return n.t.Start(ctx, spanName, opts...)
}

func (noOpTracer) Close() error {
	return nil
}

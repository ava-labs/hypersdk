// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package trace

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"

	oteltrace "go.opentelemetry.io/otel/trace"
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
	return n.t.Start(ctx, spanName, opts...) //nolint:spancheck
}

func (noOpTracer) Close() error {
	return nil
}

package trace

import (
	"github.com/ava-labs/avalanchego/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

var Noop trace.Tracer = noOpTracer{}

// noOpTracer is an implementation of trace.Tracer that does nothing.
type noOpTracer struct {
	noop.Tracer
}

func (noOpTracer) Close() error {
	return nil
}

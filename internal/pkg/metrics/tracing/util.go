package tracing

import (
	"context"

	"go.opencensus.io/trace"
)

// AddErrorEndSpan will end `span` and adds `err` to `span` iff err is not nil.
// This is a helper method to cut down on boiler plate.
func AddErrorEndSpan(ctx context.Context, span *trace.Span, err *error) {
	if *err != nil {
		span.AddAttributes(trace.StringAttribute("error", (*err).Error()))
	}
	span.End()
}

package metrics

import (
	"context"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// NewTimer creates a Float64Timer that wraps an opencensus float64 measurement.
// The time defaults to milliseconds.
func NewTimer(name, desc string) *Float64Timer {
	log.Infof("registering timer: %s - %s", name, desc)
	fMeasure := stats.Float64(name, desc, stats.UnitMilliseconds)
	fView := &view.View{
		Name:        name,
		Measure:     fMeasure,
		Description: desc,
		// [>=0ms, >=25ms, >=50ms, >=75ms, >=100ms, >=200ms, >=400ms, >=600ms, >=800ms, >=1s, >=2s, >=4s, >=8s]
		Aggregation: view.Distribution(25, 50, 75, 100, 200, 400, 600, 800, 1000, 2000, 4000, 8000),
	}
	if err := view.Register(fView); err != nil {
		// a panic here indicates a developer error when creating a view.
		// Since this method is called in init() methods, this panic when hit
		// will cause running the program to fail immediately.
		panic(err)
	}

	return &Float64Timer{
		measureMs: fMeasure,
		view:      fView,
	}
}

// Float64Timer contains a opencensus measurement and view
type Float64Timer struct {
	measureMs *stats.Float64Measure
	view      *view.View
}

// Start starts a timer and returns a Stopwatch.
func (t *Float64Timer) Start(ctx context.Context) *Stopwatch {
	return &Stopwatch{
		ctx:      ctx,
		start:    time.Now(),
		recorder: t.measureMs.M,
	}

}

// Stopwatch contains a start time and a recorder, when stopped it record the
// duration since start time began via its recorder function.
type Stopwatch struct {
	ctx      context.Context
	start    time.Time
	recorder func(v float64) stats.Measurement
}

// Stop rounds the time since Start was called to milliseconds and records the value
// in the corresponding opencensus view.
func (sw *Stopwatch) Stop(ctx context.Context) time.Duration {
	duration := time.Since(sw.start).Round(time.Millisecond)
	stats.Record(ctx, sw.recorder(float64(duration)/1e6))
	return duration
}

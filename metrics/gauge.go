package metrics

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// Int64Gauge wraps an opencensus int64 measure that is uses as a gauge.
type Int64Gauge struct {
	measureCt *stats.Int64Measure
	view      *view.View
}

// NewInt64Gauge creates a new Int64Gauge with demensionless units.
func NewInt64Gauge(name, desc string, keys ...tag.Key) *Int64Gauge {
	log.Infof("registering int64 gauge: %s - %s", name, desc)
	iMeasure := stats.Int64(name, desc, stats.UnitDimensionless)

	iView := &view.View{
		Name:        name,
		Measure:     iMeasure,
		Description: desc,
		Aggregation: view.LastValue(),
		TagKeys:     keys,
	}
	if err := view.Register(iView); err != nil {
		// a panic here indicates a developer error when creating a view.
		// Since this method is called in init() methods, this panic when hit
		// will cause running the program to fail immediately.
		panic(err)
	}

	return &Int64Gauge{
		measureCt: iMeasure,
		view:      iView,
	}
}

// Set sets the value of the gauge to value `v`.
func (c *Int64Gauge) Set(ctx context.Context, v int64) {
	stats.Record(ctx, c.measureCt.M(v))
}

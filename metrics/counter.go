package metrics

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// Int64Counter wraps an opencensus int64 measure that is uses as a counter.
type Int64Counter struct {
	measureCt *stats.Int64Measure
	view      *view.View
}

// NewInt64Counter creates a new Int64Counter with demensionless units.
func NewInt64Counter(name, desc string) *Int64Counter {
	log.Infof("registering int64 counter: %s - %s", name, desc)
	iMeasure := stats.Int64(name, desc, stats.UnitDimensionless)
	iView := &view.View{
		Name:        name,
		Measure:     iMeasure,
		Description: desc,
		Aggregation: view.Count(),
	}
	if err := view.Register(iView); err != nil {
		// a panic here indicates a developer error when creating a view.
		// Since this method is called in init() methods, this panic when hit
		// will cause running the program to fail immediately.
		panic(err)
	}

	return &Int64Counter{
		measureCt: iMeasure,
		view:      iView,
	}
}

// Inc increments the counter by value `v`.
func (c *Int64Counter) Inc(ctx context.Context, v int64) {
	stats.Record(ctx, c.measureCt.M(v))
}

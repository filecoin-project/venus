package metrics

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// NewInt64ByteBucket creates a Int64Bucket with units of bytes and a default set of aggregation
// bounds from .25 kilobytes to 10 megabytes.
func NewInt64ByteBucket(name, desc string, tagKeys ...tag.Key) *Int64Bucket {
	defaultBounds := []float64{250, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000, 5000000, 10000000}
	return NewInt64BucketWithBounds(name, desc, stats.UnitBytes, defaultBounds, tagKeys...)
}

// NewInt64BucketWithBounds creates a Int64Bucket wrapping an opencensus int64 measurement.
func NewInt64BucketWithBounds(name, desc, unit string, bounds []float64, tagKeys ...tag.Key) *Int64Bucket {
	log.Infof("registering avg counter: %s - %s", name, desc)
	measure := stats.Int64(name, desc, unit)
	iView := &view.View{
		Name:        name,
		Measure:     measure,
		Description: desc,
		TagKeys:     tagKeys,
		Aggregation: view.Distribution(bounds...),
	}
	if err := view.Register(iView); err != nil {
		// a panic here indicates a developer error when creating a view.
		// Since this method is called in init() methods, this panic when hit
		// will cause running the program to fail immediately.
		panic(err)
	}

	return &Int64Bucket{
		measureCt: measure,
		view:      iView,
	}
}

// Int64Bucket wraps an opencensus int64 measure that is uses as a distribution.
type Int64Bucket struct {
	measureCt *stats.Int64Measure
	view      *view.View
}

// Add adds `m` to the values tracked by Int64Bucket.
func (c *Int64Bucket) Add(ctx context.Context, m int64) {
	stats.Record(ctx, c.measureCt.M(m))
}

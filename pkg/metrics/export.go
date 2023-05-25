package metrics

import (
	"net/http"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	prom "github.com/prometheus/client_golang/prometheus"
	"go.opencensus.io/stats/view"
	octrace "go.opencensus.io/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/bridge/opencensus"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/filecoin-project/venus/pkg/config"
)

// RegisterPrometheusEndpoint registers and serves prometheus metrics
func RegisterPrometheusEndpoint(cfg *config.MetricsConfig) error {
	if !cfg.PrometheusEnabled {
		return nil
	}

	// validate config values and marshal to types
	interval, err := time.ParseDuration(cfg.ReportInterval)
	if err != nil {
		log.Errorf("invalid metrics interval: %s", err)
		return err
	}

	promma, err := ma.NewMultiaddr(cfg.PrometheusEndpoint)
	if err != nil {
		return err
	}

	_, promAddr, err := manet.DialArgs(promma) // nolint
	if err != nil {
		return err
	}

	// setup prometheus
	registry := prom.NewRegistry()
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "filecoin",
		Registry:  registry,
	})
	if err != nil {
		return err
	}

	view.RegisterExporter(pe)
	view.SetReportingPeriod(interval)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", pe)
		if err := http.ListenAndServe(promAddr, mux); err != nil {
			log.Errorf("failed to serve /metrics endpoint on %v", err)
		}
	}()

	return nil
}

// RegisterJaeger registers the jaeger endpoint with opencensus and names the
// tracer `serviceName`.
func RegisterJaeger(serviceName string, cfg *config.TraceConfig) (*tracesdk.TracerProvider, error) {
	if !cfg.JaegerTracingEnabled {
		return nil, nil
	}

	if len(cfg.ServerName) != 0 {
		serviceName = cfg.ServerName
	}
	je, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.JaegerEndpoint)))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(je),
		// Record information about this application in an Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	tracer := tp.Tracer(serviceName)
	octrace.DefaultTracer = opencensus.NewTracer(tracer)
	return tp, nil
}

package metrics

import (
	"net/http"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/prometheus"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	prom "github.com/prometheus/client_golang/prometheus"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/config"
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

	_, promAddr, err := manet.DialArgs(promma)
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
// tracer `name`.
func RegisterJaeger(name string, cfg *config.TraceConfig) error {
	if !cfg.JaegerTracingEnabled {
		return nil
	}
	je, err := jaeger.NewExporter(jaeger.Options{
		CollectorEndpoint: cfg.JaegerEndpoint,
		Process: jaeger.Process{
			ServiceName: name,
		},
	})
	if err != nil {
		return err
	}

	trace.RegisterExporter(je)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.ProbabilitySampler(cfg.ProbabilitySampler)})

	return nil
}

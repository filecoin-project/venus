package aggregator

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const aggregatorLabel = "aggregator"

var (
	connectedNodes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "connected_nodes",
			Help: "number of nodes connected to aggregator",
		},
		[]string{aggregatorLabel},
	)

	nodesConsensus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nodes_in_consensus",
			Help: "number of nodes in consensus",
		},
		[]string{aggregatorLabel},
	)

	nodesDispute = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nodes_in_disput",
			Help: "number of nodes in dispute",
		},
		[]string{aggregatorLabel},
	)
)

func init() {
	prometheus.MustRegister(connectedNodes, nodesConsensus, nodesDispute)
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":8080", nil) // nolint: errcheck
}

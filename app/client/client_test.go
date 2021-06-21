package client

import (
	"context"
	"contrib.go.opencensus.io/exporter/jaeger"
	"fmt"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/metrics"
	"github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/ipfs/go-cid"
	"go.opencensus.io/trace"
	"net/http"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		fmt.Printf("setup failed:%s\n", err.Error())
		return
	}

	code := m.Run()

	shutdown()

	os.Exit(code)
}

var clt *FullNodeStruct
var closer jsonrpc.ClientCloser

var endpoint = "http://localhost:3453/rpc/v0"
var token = ""

var jaegerProxyEndpoint = "192.168.1.125:6831"
var jaegerExporter *jaeger.Exporter

func setup() error {
	var err error

	if err = initClient(); err != nil {
		return err
	}

	if jaegerExporter, err = metrics.RegisterJaeger("", &config.TraceConfig{
		JaegerTracingEnabled: true,
		ProbabilitySampler:   1.0,
		JaegerEndpoint:       jaegerProxyEndpoint,
		ServerName:           "venus-client",
	}); err != nil {
		fmt.Printf("failed registerjaeger(%s), message:%s\n",
			jaegerProxyEndpoint, err.Error())
		return err
	}
	return nil
}

func TestAPIs(t *testing.T) {
	testflags.UnitTest(t)

	cid, _ := cid.Decode("bafy2bzacedylucvhzggupihcqjfvv7s4mthgetphb2zzlp7kpjpykmiklfzt4")

	ctx, span := trace.StartSpan(context.TODO(), "test_api")
	defer span.End()

	sctx := span.SpanContext()
	span.AddAttributes(trace.StringAttribute("user", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJhZG1pbiJdfQ.8KBg6HE9KZFxaPogQhYCAN2HNcK5lSR37NMRrem5aVY"))

	fmt.Printf("tracid:%s, spanid:%s\n", sctx.TraceID, sctx.SpanID)

	if _, err := clt.ChainGetBlock(ctx, cid); err != nil {
		fmt.Printf("call chainhead failed:%s\n", err.Error())
	}
}

func initClient() error {
	node := FullNodeStruct{}
	headers := http.Header{}

	if len(token) != 0 {
		headers.Add("Authorization", "Bearer "+token)
	}

	var err error
	if closer, err = jsonrpc.NewClient(context.TODO(), endpoint, "Filecoin", &node, headers); err == nil {
		clt = &node
	}
	return err
}

func shutdown() {
	if jaegerExporter != nil {
		jaegerExporter.Flush()
	}

	if closer != nil {
		closer()
	}
}

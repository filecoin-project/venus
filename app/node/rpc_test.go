package node

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/filecoin-project/venus/app/client/funcrule"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/filecoin-project/go-jsonrpc"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestWsBuilder(t *testing.T) {
	tf.UnitTest(t)

	nameSpace := "Test"
	builder := NewBuilder().NameSpace(nameSpace)
	err := builder.AddServices(&tmodule1{}, &tmodule2{})
	require.NoError(t, err)

	server := mockBuild(builder)
	testServ := httptest.NewServer(server)
	defer testServ.Close()
	var client FullAdapter
	closer, err := jsonrpc.NewClient(
		context.Background(),
		"ws://"+testServ.Listener.Addr().String(),
		nameSpace,
		&client,
		nil)
	require.NoError(t, err)
	defer closer()

	result, err := client.Test1(context.Background())
	require.NoError(t, err)
	assert.Equal(t, result, "test")
}

func TestJsonrpc(t *testing.T) {
	tf.UnitTest(t)

	nameSpace := "Test"
	builder := NewBuilder().NameSpace(nameSpace)
	err := builder.AddService(&tmodule1{})
	require.NoError(t, err)

	server := mockBuild(builder)
	testServ := httptest.NewServer(server)
	defer testServ.Close()

	http.Handle("/rpc/v1", server)

	req := struct {
		Jsonrpc string            `json:"jsonrpc"`
		ID      int64             `json:"id,omitempty"`
		Method  string            `json:"method"`
		Meta    map[string]string `json:"meta,omitempty"`
	}{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "Test.Test1",
	}
	reqBytes, err := json.Marshal(req)
	require.NoError(t, err)
	httpRes, err := http.Post("http://"+testServ.Listener.Addr().String()+"/rpc/v1", "", bytes.NewReader(reqBytes))
	require.NoError(t, err)
	assert.Equal(t, httpRes.Status, "200 OK")
	result, err := ioutil.ReadAll(httpRes.Body)
	require.NoError(t, err)
	res := struct {
		Result string `json:"result"`
	}{}
	err = json.Unmarshal(result, &res)
	require.NoError(t, err)
	assert.Equal(t, res.Result, "test")
}

type tmodule1 struct {
}

func (m *tmodule1) V0API() MockAPI1 { //nolint
	return &mockAPI1{}
}

func (m *tmodule1) API() MockAPI1 { //nolint
	return &mockAPI1{}
}

type tmodule2 struct {
}

func (m *tmodule2) V0API() MockAPI2 { //nolint
	return &mockAPI2{}
}

func (m *tmodule2) API() MockAPI2 { //nolint
	return &mockAPI2{}
}

var _ MockAPI1 = &mockAPI1{}

type MockAPI1 interface {
	Test1(ctx context.Context) (string, error)
}

type MockAPI2 interface {
	Test2(ctx context.Context) error
}
type mockAPI1 struct {
}

func (m *mockAPI1) Test1(ctx context.Context) (string, error) {
	return "test", nil
}

var _ MockAPI2 = &mockAPI2{}

type mockAPI2 struct {
}

func (m *mockAPI2) Test2(ctx context.Context) error {
	return nil
}

type FullAdapter struct {
	CommonAdapter
	Adapter2
}
type CommonAdapter struct {
	Adapter1
}
type Adapter1 struct {
	Test1 func(ctx context.Context) (string, error) `perm:"read"`
}

type Adapter2 struct {
	Test2 func(ctx context.Context) error `perm:"read"`
}

func mockBuild(builder *RPCBuilder) *jsonrpc.RPCServer {
	server := jsonrpc.NewServer(jsonrpc.WithProxyBind(jsonrpc.PBField))
	var fullNode FullAdapter
	for _, apiStruct := range builder.v1APIStruct {
		funcrule.PermissionProxy(apiStruct, &fullNode)
	}
	for _, nameSpace := range builder.namespace {
		server.Register(nameSpace, &fullNode)
	}
	return server
}

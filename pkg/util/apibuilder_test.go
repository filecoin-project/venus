package util

import (
	"bytes"
	"encoding/json"
	"github.com/filecoin-project/go-jsonrpc"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWsBuilder(t *testing.T) {
	tf.UnitTest(t)
	nameSpace := "Test"
	builder := NewBuiler().NameSpace(nameSpace)
	err := builder.AddService(&tmodule{})
	require.NoError(t, err)
	server := builder.Build()

	testServ := httptest.NewServer(server)
	defer testServ.Close()
	var client struct {
		Test func() (string, error)
	}

	closer, err := jsonrpc.NewClient("ws://"+testServ.Listener.Addr().String(), nameSpace, &client, nil)
	require.NoError(t, err)
	defer closer()

	result, err := client.Test()
	require.NoError(t, err)
	assert.Equal(t, result, "test")
}

func TestJsonrpc(t *testing.T) {
	tf.UnitTest(t)
	nameSpace := "Test"
	builder := NewBuiler().NameSpace(nameSpace)
	err := builder.AddService(&tmodule{})
	require.NoError(t, err)
	server := builder.Build()

	testServ := httptest.NewServer(server)
	defer testServ.Close()

	http.Handle("/rpc/v0", server)

	req := struct {
		Jsonrpc string            `json:"jsonrpc"`
		ID      int64             `json:"id,omitempty"`
		Method  string            `json:"method"`
		Meta    map[string]string `json:"meta,omitempty"`
	}{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "Test.Test",
	}
	reqBytes, err := json.Marshal(req)
	require.NoError(t, err)
	httpRes, err := http.Post("http://"+testServ.Listener.Addr().String()+"/rpc/v0", "", bytes.NewReader(reqBytes))
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

type tmodule struct {
}

func (m *tmodule) API() interface{} { //nolint
	return &mockAPI{}
}

type mockAPI struct {
}

func (m *mockAPI) Test() (string, error) {
	return "test", nil
}

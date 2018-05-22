package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

// A MockEmitter satisfies the ValueEmitter interface and records its calls.
type MockEmitter struct {
	emitterFunc func(value interface{}) error
	captures    *[]interface{}
}

// NewMockEmitter creates a MockEmitter from the provided emitFunc.
func NewMockEmitter(emitFunc func(interface{}) error) *MockEmitter {
	return &MockEmitter{
		emitterFunc: emitFunc,
		captures:    &[]interface{}{},
	}
}

func (ce MockEmitter) emit(value interface{}) error {
	*ce.captures = append(*ce.captures, value)
	return ce.emitterFunc(value)
}

func (ce MockEmitter) calls() []interface{} {
	return *ce.captures
}

func requireSchemaConformance(t *testing.T, jsonBytes []byte, schemaName string) { // nolint: deadcode
	wdir, _ := os.Getwd()
	rLoader := gojsonschema.NewReferenceLoader(fmt.Sprintf("file://%s/schema/%s.schema.json", wdir, schemaName))
	jLoader := gojsonschema.NewBytesLoader(jsonBytes)

	result, err := gojsonschema.Validate(rLoader, jLoader)
	require.NoError(t, err)

	for _, desc := range result.Errors() {
		t.Errorf("- %s\n", desc)
	}

	require.Truef(t, result.Valid(), "Error schema validating: %s", string(jsonBytes))
}

//GetFilecoinBinary returns the path where the filecoin binary will be if it has been built.
func GetFilecoinBinary() (string, error) {
	bin := filepath.FromSlash(fmt.Sprintf("%s/src/github.com/filecoin-project/go-filecoin/go-filecoin", os.Getenv("GOPATH")))
	_, err := os.Stat(bin)
	if err == nil {
		return bin, nil
	}

	if os.IsNotExist(err) {
		return "", fmt.Errorf("You are missing the filecoin binary...try building, searched in '%s'", bin)
	}

	return "", err
}

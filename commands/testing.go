package commands

import (
	"fmt"
	"os"
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

	require.True(t, result.Valid())
}

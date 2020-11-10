package dispatch

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/internal/pkg/encoding"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

type fakeActor struct{}

type SimpleParams struct {
	Name string
}

type SimpleReturn struct {
	someValue uint64
}

func (*fakeActor) simpleMethod(ctx interface{}, params SimpleParams) SimpleReturn {
	return SimpleReturn{someValue: 3}
}

func (*fakeActor) pointerParam(ctx interface{}, params *SimpleParams) SimpleReturn {
	return SimpleReturn{someValue: 3}
}

func (*fakeActor) pointerReturn(ctx interface{}, params SimpleParams) *SimpleReturn {
	return &SimpleReturn{someValue: 3}
}

func (*fakeActor) noParams(ctx interface{}) SimpleReturn {
	return SimpleReturn{someValue: 3}
}

func (*fakeActor) noReturn(ctx interface{}, params *SimpleParams) {
	/* empty */
}

func (*fakeActor) minimalist(ctx interface{}) {
	/* empty */
}

func TestArgInterface(t *testing.T) {
	tf.UnitTest(t)

	fa := fakeActor{}

	params := SimpleParams{Name: "tester"}
	setup := func(method interface{}) (methodSignature, []byte) {
		s := methodSignature{method: reflect.ValueOf(method)}

		encodedParams, err := encoding.Encode(params)
		assert.NoError(t, err)

		return s, encodedParams
	}

	assertArgInterface := func(s methodSignature, encodedParams []byte) interface{} {
		ret, err := s.ArgInterface(encodedParams)
		assert.NoError(t, err)
		assert.NotNil(t, ret)
		return ret
	}

	t.Run("simpleMethod", func(t *testing.T) {
		s, encodedParams := setup(fa.simpleMethod)

		ret := assertArgInterface(s, encodedParams)

		v, ok := ret.(SimpleParams)
		assert.True(t, ok)
		assert.Equal(t, params.Name, v.Name)
	})

	t.Run("pointerParam", func(t *testing.T) {
		s, encodedParams := setup(fa.pointerParam)

		ret := assertArgInterface(s, encodedParams)

		v, ok := ret.(*SimpleParams)
		assert.True(t, ok)
		assert.Equal(t, params.Name, v.Name)
	})

	t.Run("noParams", func(t *testing.T) {
		// Dragons: not supported, must panic
	})
}

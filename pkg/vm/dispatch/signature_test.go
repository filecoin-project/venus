package dispatch

import (
	"bytes"
	"reflect"
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

type fakeActor struct{}

type SimpleReturn struct {
	someValue uint64
}

func (*fakeActor) pointerParam(ctx interface{}, params *SimpleParams) SimpleReturn {
	return SimpleReturn{someValue: 3}
}

func TestArgInterface(t *testing.T) {
	tf.UnitTest(t)

	fa := fakeActor{}

	params := SimpleParams{Name: "tester"}
	setup := func(method interface{}) (methodSignature, []byte) {
		s := methodSignature{method: reflect.ValueOf(method)}

		buf := new(bytes.Buffer)
		err := params.MarshalCBOR(buf)
		assert.NoError(t, err)

		return s, buf.Bytes()
	}

	assertArgInterface := func(s methodSignature, encodedParams []byte) interface{} {
		ret, err := s.ArgInterface(encodedParams)
		assert.NoError(t, err)
		assert.NotNil(t, ret)
		return ret
	}

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

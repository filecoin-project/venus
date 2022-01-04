package testutil

import (
	"fmt"
	"math/rand"
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/require"
)

func TestValueSetNReset(t *testing.T) {
	tf.UnitTest(t)

	for i := 0; i < 32; i++ {
		originVal := rand.Int()
		newVal := originVal + 1

		target := originVal
		ValueSetNReset(t, fmt.Sprintf("set %d to %d", originVal, newVal), func() { require.Equal(t, target, newVal, "after set") }, func() { require.Equal(t, target, originVal, "after reset") }, &target, newVal)
	}
}

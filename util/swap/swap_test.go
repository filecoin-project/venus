package swap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwap(t *testing.T) {
	assert := assert.New(t)

	age := 25
	fn := func() int { return 1 }
	func() {
		assert.Equal(25, age)
		assert.Equal(1, fn())
		defer Swap(&age, 39)()
		defer Swap(&fn, func() int { return 2 })()
		assert.Equal(39, age)
		assert.Equal(2, fn())
	}()
	assert.Equal(25, age)
	assert.Equal(1, fn())
}

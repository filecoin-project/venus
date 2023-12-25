package splitstore

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfinityChannel(t *testing.T) {
	in, out := NewInfinityChannel[int]()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0

		for v := range out {
			require.Equal(t, i, v)
			i++
		}
	}()

	for i := 0; i < 100; i++ {
		in <- i
	}

	close(in)
	wg.Wait()
}

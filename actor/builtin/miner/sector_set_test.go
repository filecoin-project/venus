package miner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestSectorSet(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Add", func(t *testing.T) {
		ss := NewSectorSet()
		comm1 := th.MakeCommitments()
		comm2 := th.MakeCommitments()
		ss.Add(1, comm1)
		ss.Add(3, comm2)
		assert.Equal(t, 2, len(ss))
		for k, v := range ss {
			if k == "1" {
				assert.Equal(t, comm1, v)
			}
			if k == "3" {
				assert.Equal(t, comm2, v)
			}
		}
	})

	t.Run("Has", func(t *testing.T) {
		ss := NewSectorSet()
		comm1 := th.MakeCommitments()
		ss.Add(1, comm1)
		assert.True(t, ss.Has(1))
		assert.False(t, ss.Has(2))
	})

	t.Run("Get", func(t *testing.T) {
		ss := NewSectorSet()
		comm1 := th.MakeCommitments()
		comm2 := th.MakeCommitments()
		ss.Add(1, comm1)
		ss.Add(3, comm2)
		commRet, ok := ss.Get(1)
		assert.True(t, ok)
		assert.Equal(t, comm1, commRet)

		_, ok = ss.Get(2)
		assert.False(t, ok)
	})

	t.Run("Drop", func(t *testing.T) {
		ss := NewSectorSet()
		comm1 := th.MakeCommitments()
		comm2 := th.MakeCommitments()
		comm3 := th.MakeCommitments()
		comm4 := th.MakeCommitments()
		comm5 := th.MakeCommitments()

		ss.Add(1, comm1)
		ss.Add(2, comm2)
		ss.Add(3, comm3)
		ss.Add(5, comm4)
		ss.Add(8, comm5)

		assert.NoError(t, ss.Drop([]uint64{5, 1, 2}))
		assert.True(t, ss.Has(3))
		assert.True(t, ss.Has(8))
		assert.False(t, ss.Has(1))
		assert.False(t, ss.Has(2))
		assert.False(t, ss.Has(5))
	})

	t.Run("IDs", func(t *testing.T) {
		ss := NewSectorSet()
		comm1 := th.MakeCommitments()
		comm2 := th.MakeCommitments()
		comm3 := th.MakeCommitments()
		comm4 := th.MakeCommitments()
		comm5 := th.MakeCommitments()

		ss.Add(1, comm1)
		ss.Add(2, comm2)
		ss.Add(3, comm3)
		ss.Add(5, comm4)
		ss.Add(8, comm5)

		ids, err := ss.IDs()
		assert.NoError(t, err)
		assert.Equal(t, 5, len(ids))
		assert.Contains(t, ids, uint64(1))
		assert.Contains(t, ids, uint64(2))
		assert.Contains(t, ids, uint64(3))
		assert.Contains(t, ids, uint64(5))
		assert.Contains(t, ids, uint64(8))
	})
}

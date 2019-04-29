package errors

import (
	"testing"

	"github.com/pkg/errors"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestFaultError(t *testing.T) {
	tf.UnitTest(t)

	assert.Contains(t, NewFaultErrorf("%d", 42).Error(), "42")
	assert.True(t, IsFault(NewFaultError("boom")))

	err := errors.New("source")
	assert.False(t, IsFault(err))
	fe := FaultErrorWrap(err, "msg")
	assert.True(t, IsFault(fe))
	assert.Contains(t, fe.Error(), "source")
	assert.Contains(t, fe.Error(), "msg")
	wrapped := errors.Wrap(fe, "wrapped")
	assert.True(t, IsFault(wrapped))
	assert.Contains(t, wrapped.Error(), "wrapped")
	wrapped2 := errors.Wrap(wrapped, "wrapped2")
	assert.True(t, IsFault(wrapped2))
	assert.Equal(t, fe, errors.Cause(wrapped2))
}

func TestRevertError(t *testing.T) {
	tf.UnitTest(t)

	assert.Contains(t, NewRevertErrorf("%d", 42).Error(), "42")
	assert.Contains(t, RevertErrorWrapf(errors.New(""), "%d", 42).Error(), "42")
	assert.True(t, ShouldRevert(NewRevertError("boom")))

	err := errors.New("source")
	assert.False(t, ShouldRevert(err))
	re := RevertErrorWrap(err, "msg")
	assert.True(t, ShouldRevert(re))
	assert.Contains(t, re.Error(), "source")
	assert.Contains(t, re.Error(), "msg")
	wrapped := errors.Wrap(re, "wrapped")
	assert.True(t, ShouldRevert(wrapped))
	assert.Contains(t, wrapped.Error(), "wrapped")
	wrapped2 := errors.Wrap(wrapped, "wrapped2")
	assert.True(t, ShouldRevert(wrapped2))
	assert.Equal(t, re, errors.Cause(wrapped2))
}

func TestApplyErrorPermanent(t *testing.T) {
	tf.UnitTest(t)

	t.Run("random errors dont satisfy", func(t *testing.T) {
		assert.False(t, IsApplyErrorPermanent(errors.New("boom")))
	})
	t.Run("perm errors do satisfy", func(t *testing.T) {
		assert.True(t, IsApplyErrorPermanent(ApplyErrorPermanentWrapf(errors.New("boom"), "wrapper")))
	})
	t.Run("message has both wrapped and wrapper and format string works", func(t *testing.T) {
		e := ApplyErrorPermanentWrapf(errors.New("wrapped"), "%s", "wrapper")
		assert.Contains(t, e.Error(), "wrapped")
		assert.Contains(t, e.Error(), "wrapper")
	})
	t.Run("Cause is the wrapped error", func(t *testing.T) {
		e := ApplyErrorPermanentWrapf(errors.New("wrapped"), "wrapper")
		assert.Contains(t, errors.Cause(e).Error(), "wrapped")
		assert.NotContains(t, errors.Cause(e).Error(), "wrapper")
	})
}

func TestApplyErrorTemporary(t *testing.T) {
	tf.UnitTest(t)

	t.Run("random errors dont satisfy", func(t *testing.T) {
		assert.False(t, IsApplyErrorTemporary(errors.New("boom")))
	})
	t.Run("temp errors do satisfy", func(t *testing.T) {
		assert.True(t, IsApplyErrorTemporary(ApplyErrorTemporaryWrapf(errors.New("boom"), "wrapper")))
	})
	t.Run("message has both wrapped and wrapper and format string works", func(t *testing.T) {
		e := ApplyErrorTemporaryWrapf(errors.New("wrapped"), "%s", "wrapper")
		assert.Contains(t, e.Error(), "wrapped")
		assert.Contains(t, e.Error(), "wrapper")
	})
	t.Run("Cause is the wrapped error", func(t *testing.T) {
		e := ApplyErrorTemporaryWrapf(errors.New("wrapped"), "wrapper")
		assert.Contains(t, errors.Cause(e).Error(), "wrapped")
		assert.NotContains(t, errors.Cause(e).Error(), "wrapper")
	})
}

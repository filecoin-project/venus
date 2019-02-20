package errors

import (
	"testing"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestFaultError(t *testing.T) {
	assert := assert.New(t)

	assert.Contains(NewFaultErrorf("%d", 42).Error(), "42")
	assert.True(IsFault(NewFaultError("boom")))

	err := errors.New("source")
	assert.False(IsFault(err))
	fe := FaultErrorWrap(err, "msg")
	assert.True(IsFault(fe))
	assert.Contains(fe.Error(), "source")
	assert.Contains(fe.Error(), "msg")
	wrapped := errors.Wrap(fe, "wrapped")
	assert.True(IsFault(wrapped))
	assert.Contains(wrapped.Error(), "wrapped")
	wrapped2 := errors.Wrap(wrapped, "wrapped2")
	assert.True(IsFault(wrapped2))
	assert.Equal(fe, errors.Cause(wrapped2))
}

func TestRevertError(t *testing.T) {
	assert := assert.New(t)

	assert.Contains(NewRevertErrorf("%d", 42).Error(), "42")
	assert.Contains(RevertErrorWrapf(errors.New(""), "%d", 42).Error(), "42")
	assert.True(ShouldRevert(NewRevertError("boom")))

	err := errors.New("source")
	assert.False(ShouldRevert(err))
	re := RevertErrorWrap(err, "msg")
	assert.True(ShouldRevert(re))
	assert.Contains(re.Error(), "source")
	assert.Contains(re.Error(), "msg")
	wrapped := errors.Wrap(re, "wrapped")
	assert.True(ShouldRevert(wrapped))
	assert.Contains(wrapped.Error(), "wrapped")
	wrapped2 := errors.Wrap(wrapped, "wrapped2")
	assert.True(ShouldRevert(wrapped2))
	assert.Equal(re, errors.Cause(wrapped2))
}

func TestApplyErrorPermanent(t *testing.T) {
	t.Run("random errors dont satisfy", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(IsApplyErrorPermanent(errors.New("boom")))
	})
	t.Run("perm errors do satisfy", func(t *testing.T) {
		assert := assert.New(t)
		assert.True(IsApplyErrorPermanent(ApplyErrorPermanentWrapf(errors.New("boom"), "wrapper")))
	})
	t.Run("message has both wrapped and wrapper and format string works", func(t *testing.T) {
		assert := assert.New(t)
		e := ApplyErrorPermanentWrapf(errors.New("wrapped"), "%s", "wrapper")
		assert.Contains(e.Error(), "wrapped")
		assert.Contains(e.Error(), "wrapper")
	})
	t.Run("Cause is the wrapped error", func(t *testing.T) {
		assert := assert.New(t)
		e := ApplyErrorPermanentWrapf(errors.New("wrapped"), "wrapper")
		assert.Contains(errors.Cause(e).Error(), "wrapped")
		assert.NotContains(errors.Cause(e).Error(), "wrapper")
	})
}

func TestApplyErrorTemporary(t *testing.T) {
	t.Run("random errors dont satisfy", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(IsApplyErrorTemporary(errors.New("boom")))
	})
	t.Run("temp errors do satisfy", func(t *testing.T) {
		assert := assert.New(t)
		assert.True(IsApplyErrorTemporary(ApplyErrorTemporaryWrapf(errors.New("boom"), "wrapper")))
	})
	t.Run("message has both wrapped and wrapper and format string works", func(t *testing.T) {
		assert := assert.New(t)
		e := ApplyErrorTemporaryWrapf(errors.New("wrapped"), "%s", "wrapper")
		assert.Contains(e.Error(), "wrapped")
		assert.Contains(e.Error(), "wrapper")
	})
	t.Run("Cause is the wrapped error", func(t *testing.T) {
		assert := assert.New(t)
		e := ApplyErrorTemporaryWrapf(errors.New("wrapped"), "wrapper")
		assert.Contains(errors.Cause(e).Error(), "wrapped")
		assert.NotContains(errors.Cause(e).Error(), "wrapper")
	})
}

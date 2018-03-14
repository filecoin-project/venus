package core

import (
	"testing"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/stretchr/testify/assert"
)

func TestFaultError(t *testing.T) {
	assert := assert.New(t)

	assert.Contains(newFaultErrorf("%d", 42).Error(), "42")
	assert.True(IsFault(newFaultError("boom")))

	err := errors.New("source")
	assert.False(IsFault(err))
	fe := faultErrorWrap(err, "msg")
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

	assert.Contains(newRevertErrorf("%d", 42).Error(), "42")
	assert.Contains(revertErrorWrapf(errors.New(""), "%d", 42).Error(), "42")
	assert.True(shouldRevert(newRevertError("boom")))

	err := errors.New("source")
	assert.False(shouldRevert(err))
	re := revertErrorWrap(err, "msg")
	assert.True(shouldRevert(re))
	assert.Contains(re.Error(), "source")
	assert.Contains(re.Error(), "msg")
	wrapped := errors.Wrap(re, "wrapped")
	assert.True(shouldRevert(wrapped))
	assert.Contains(wrapped.Error(), "wrapped")
	wrapped2 := errors.Wrap(wrapped, "wrapped2")
	assert.True(shouldRevert(wrapped2))
	assert.Equal(re, errors.Cause(wrapped2))
}

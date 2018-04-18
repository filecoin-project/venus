package core

import (
	"fmt"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

var (
	// Most errors should live in the actors that throw them. However some
	// errors will be pervasive so we define them centrally here.

	// ErrCannotTransferNegativeValue signals a transfer error, value must be positive.
	ErrCannotTransferNegativeValue = newRevertError("cannot transfer negative values")
	// ErrInsufficientBalance signals insufficient balance for a transfer.
	ErrInsufficientBalance = newRevertError("not enough balance")
)

// revertError is an error wrapper that signals that the vm should
// revert all state changes for the call.
//
// Note that this error wrapping intentionally does not implement
// Cause() so it becomes the root Cause() of any outer wrappings.
// If you wrap revertError or faultErrorWrap in another one
// of those two, it effectively masks the inner one from Cause().
// Doesn't seem like a big deal but if we want to we could
// have a constructor that checked for this or do more elaborate
// wrapping (basically duplicating what's in errors.Wrap).
type revertError struct {
	err error
	msg string
}

func (re revertError) Error() string {
	if re.err == nil {
		return re.msg
	}
	return fmt.Sprintf("%s: %s", re.msg, re.err.Error())
}

func (re revertError) ShouldRevert() bool {
	return true
}

func newRevertError(msg string) error {
	return &revertError{err: nil, msg: msg}
}

func newRevertErrorf(format string, args ...interface{}) error {
	return newRevertError(fmt.Sprintf(format, args...))
}

func revertErrorWrap(err error, msg string) error {
	return &revertError{err: err, msg: msg}
}

func revertErrorWrapf(err error, format string, args ...interface{}) error { // nolint: deadcode
	return &revertError{err: err, msg: fmt.Sprintf(format, args...)}
}

type reverterror interface {
	ShouldRevert() bool
}

// shouldRevert indicates if we should revert. It looks at the
// root Cause() to make that judgement.
func shouldRevert(err error) bool {
	cause := errors.Cause(err)
	re, ok := cause.(reverterror)
	return ok && re.ShouldRevert()
}

// faultError is an error wrapper that signifies a system fault (corrupted
// disk or similar). Not only should state changes be reverted but
// processing should stop.
type faultError struct {
	err error
	msg string
}

func (fe faultError) Error() string {
	if fe.err == nil {
		return fe.msg
	}
	return fmt.Sprintf("%s: %s", fe.msg, fe.err.Error())
}

func (fe faultError) IsFault() bool {
	return true
}

func newFaultError(msg string) error {
	return &faultError{err: nil, msg: msg}
}

func newFaultErrorf(format string, args ...interface{}) error {
	return newFaultError(fmt.Sprintf(format, args...))
}

func faultErrorWrap(err error, msg string) error {
	return &faultError{err: err, msg: msg}
}

func faultErrorWrapf(err error, format string, args ...interface{}) error {
	return &faultError{err: err, msg: fmt.Sprintf(format, args...)}
}

type faulterror interface {
	IsFault() bool
}

// IsFault indicates a system fault. If it returns true the
// code should bug out fast -- something is badly broken.
// IsFault looks at the root Cause() to make that judgement.
func IsFault(err error) bool {
	cause := errors.Cause(err)
	fe, ok := cause.(faulterror)
	return ok && fe.IsFault()
}

// IsApplyErrorPermanent returns true if the error returned by ApplyMessage is
// a permanent failure, the message likely will never result in a valid state
// transition (eg, trying to send negative value).
func IsApplyErrorPermanent(err error) bool {
	// Note: does not look at cause.
	re, ok := err.(permanent)
	return ok && re.IsPermanent()
}

type applyErrorPermanent struct {
	err error
	msg string
}

func (e applyErrorPermanent) Error() string {
	if e.err == nil {
		return e.msg
	}
	return fmt.Sprintf("%s: %s", e.msg, e.err.Error())
}

func (e applyErrorPermanent) Cause() error {
	if e.err != nil {
		return e.err
	}
	return e
}

func (e applyErrorPermanent) IsPermanent() bool {
	return true
}

func applyErrorPermanentWrapf(err error, format string, args ...interface{}) error { // nolint: deadcode
	return &applyErrorPermanent{err: err, msg: fmt.Sprintf(format, args...)}
}

type permanent interface {
	IsPermanent() bool
}

// IsApplyErrorTemporary returns true if the error returned by ApplyMessage is
// possibly a temporary failure, ie the message might result in a valid state
// transition in the future (eg, nonce is too high).
func IsApplyErrorTemporary(err error) bool {
	// Note: does not look at cause.
	re, ok := err.(temporary)
	return ok && re.IsTemporary()
}

type applyErrorTemporary struct {
	err error
	msg string
}

func (e applyErrorTemporary) Error() string {
	if e.err == nil {
		return e.msg
	}
	return fmt.Sprintf("%s: %s", e.msg, e.err.Error())
}

func (e applyErrorTemporary) Cause() error {
	if e.err != nil {
		return e.err
	}
	return e
}

func (e applyErrorTemporary) IsTemporary() bool {
	return true
}

func applyErrorTemporaryWrapf(err error, format string, args ...interface{}) error { // nolint: deadcode
	return &applyErrorTemporary{err: err, msg: fmt.Sprintf(format, args...)}
}

type temporary interface {
	IsTemporary() bool
}

// Package errors implements error wrappers to signal the different
// states that VM execution can halt.
package errors

import (
	"fmt"
	"github.com/pkg/errors"
)

// RevertError is an error wrapper that signals that the vm should
// revert all state changes for the call.
//
// Note that this error wrapping intentionally does not implement
// Cause() so it becomes the root Cause() of any outer wrappings.
// If you wrap RevertError or faultErrorWrap in another one
// of those two, it effectively masks the inner one from Cause().
// Doesn't seem like a big deal but if we want to we could
// have a constructor that checked for this or do more elaborate
// wrapping (basically duplicating what's in errors.Wrap).
type RevertError struct {
	err  error
	msg  string
	code uint8
}

func (re RevertError) Error() string {
	if re.err == nil {
		return re.msg
	}
	return fmt.Sprintf("%s: %s", re.msg, re.err.Error())
}

// ShouldRevert implements the reverterror interface.
func (re RevertError) ShouldRevert() bool {
	return true
}

// NewRevertError creates a new RevertError using the passed in message.
func NewRevertError(msg string) error {
	return &RevertError{err: nil, msg: msg, code: 1}
}

// NewRevertErrorf creates a new RevertError, but with Sprintf formatting.
func NewRevertErrorf(format string, args ...interface{}) error {
	return NewRevertError(fmt.Sprintf(format, args...))
}

// NewCodedRevertError creates a new RevertError using the passed in message.
func NewCodedRevertError(code uint8, msg string) error {
	return &RevertError{err: nil, msg: msg, code: code}
}

// NewCodedRevertErrorf creates a new RevertError, but with Sprintf formatting.
func NewCodedRevertErrorf(code uint8, format string, args ...interface{}) error {
	return NewCodedRevertError(code, fmt.Sprintf(format, args...))
}

// RevertErrorWrap wraps a given error in a RevertError.
func RevertErrorWrap(err error, msg string) error {
	return &RevertError{err: err, msg: msg, code: 1}
}

// RevertErrorWrapf wraps a given error in a RevertError and adds a message
// using Sprintf formatting.
func RevertErrorWrapf(err error, format string, args ...interface{}) error { // nolint: deadcode
	return &RevertError{err: err, msg: fmt.Sprintf(format, args...), code: 1}
}

// Code returns the error code for this error
func (re RevertError) Code() uint8 {
	return re.code
}

type reverterror interface {
	ShouldRevert() bool
}

// ShouldRevert indicates if we should revert. It looks at the
// root Cause() to make that judgement.
func ShouldRevert(err error) bool {
	cause := errors.Cause(err)
	re, ok := cause.(reverterror)
	return ok && re.ShouldRevert()
}

// CodeError returns the RevertError's error code if it is a revert error, or 1 otherwise.
func CodeError(err error) uint8 {
	if err == nil {
		return 0
	}
	if ShouldRevert(err) {
		return err.(*RevertError).Code()
	}
	return 1
}

// FaultError is an error wrapper that signifies a system fault (corrupted
// disk or similar). Not only should state changes be reverted but
// processing should stop.
type FaultError struct {
	err error
	msg string
}

func (fe FaultError) Error() string {
	if fe.err == nil {
		return fe.msg
	}
	return fmt.Sprintf("%s: %s", fe.msg, fe.err.Error())
}

// IsFault implements the faulterror interface.
func (fe FaultError) IsFault() bool {
	return true
}

// NewFaultError creates a new FaultError using the passed in message.
func NewFaultError(msg string) error {
	return &FaultError{err: nil, msg: msg}
}

// NewFaultErrorf creates a new FaultError, but with Sprintf formatting.
func NewFaultErrorf(format string, args ...interface{}) error {
	return NewFaultError(fmt.Sprintf(format, args...))
}

// FaultErrorWrap wraps a given error in a FaultError.
func FaultErrorWrap(err error, msg string) error {
	return &FaultError{err: err, msg: msg}
}

// FaultErrorWrapf wraps a given error in a FaultError and adds a message
// using Sprintf formatting.
func FaultErrorWrapf(err error, format string, args ...interface{}) error {
	return &FaultError{err: err, msg: fmt.Sprintf(format, args...)}
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

// ApplyErrorPermanent is an error indicating a permanent failure.
type ApplyErrorPermanent struct {
	err error
	msg string
}

func (e ApplyErrorPermanent) Error() string {
	if e.err == nil {
		return e.msg
	}
	return fmt.Sprintf("%s: %s", e.msg, e.err.Error())
}

// Cause returns the underlying error.
func (e ApplyErrorPermanent) Cause() error {
	if e.err != nil {
		return e.err
	}
	return e
}

// IsPermanent implement the permanent interface.
func (e ApplyErrorPermanent) IsPermanent() bool {
	return true
}

// ApplyErrorPermanentWrapf wraps a given error in a ApplyErrorPermanent and adds a message
// using Sprintf formatting.
func ApplyErrorPermanentWrapf(err error, format string, args ...interface{}) error { // nolint: deadcode
	return &ApplyErrorPermanent{err: err, msg: fmt.Sprintf(format, args...)}
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

// ApplyErrorTemporary indicates a temporary failure.
type ApplyErrorTemporary struct {
	err error
	msg string
}

func (e ApplyErrorTemporary) Error() string {
	if e.err == nil {
		return e.msg
	}
	return fmt.Sprintf("%s: %s", e.msg, e.err.Error())
}

// Cause returns the underlying error.
func (e ApplyErrorTemporary) Cause() error {
	if e.err != nil {
		return e.err
	}
	return e
}

// IsTemporary implements the temporary interface.
func (e ApplyErrorTemporary) IsTemporary() bool {
	return true
}

// ApplyErrorTemporaryWrapf wraps a given error in a ApplyErrorTemporary and adds a message
// using Sprintf formatting.
func ApplyErrorTemporaryWrapf(err error, format string, args ...interface{}) error { // nolint: deadcode
	return &ApplyErrorTemporary{err: err, msg: fmt.Sprintf(format, args...)}
}

type temporary interface {
	IsTemporary() bool
}

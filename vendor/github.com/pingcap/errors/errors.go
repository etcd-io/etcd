// Package errors provides simple error handling primitives.
//
// The traditional error handling idiom in Go is roughly akin to
//
//     if err != nil {
//             return err
//     }
//
// which applied recursively up the call stack results in error reports
// without context or debugging information. The errors package allows
// programmers to add context to the failure path in their code in a way
// that does not destroy the original value of the error.
//
// Adding context to an error
//
// The errors.Annotate function returns a new error that adds context to the
// original error by recording a stack trace at the point Annotate is called,
// and the supplied message. For example
//
//     _, err := ioutil.ReadAll(r)
//     if err != nil {
//             return errors.Annotate(err, "read failed")
//     }
//
// If additional control is required the errors.AddStack and errors.WithMessage
// functions destructure errors.Annotate into its component operations of annotating
// an error with a stack trace and an a message, respectively.
//
// Retrieving the cause of an error
//
// Using errors.Annotate constructs a stack of errors, adding context to the
// preceding error. Depending on the nature of the error it may be necessary
// to reverse the operation of errors.Annotate to retrieve the original error
// for inspection. Any error value which implements this interface
//
//     type causer interface {
//             Cause() error
//     }
//
// can be inspected by errors.Cause. errors.Cause will recursively retrieve
// the topmost error which does not implement causer, which is assumed to be
// the original cause. For example:
//
//     switch err := errors.Cause(err).(type) {
//     case *MyError:
//             // handle specifically
//     default:
//             // unknown error
//     }
//
// causer interface is not exported by this package, but is considered a part
// of stable public API.
// errors.Unwrap is also available: this will retrieve the next error in the chain.
//
// Formatted printing of errors
//
// All error values returned from this package implement fmt.Formatter and can
// be formatted by the fmt package. The following verbs are supported
//
//     %s    print the error. If the error has a Cause it will be
//           printed recursively
//     %v    see %s
//     %+v   extended format. Each Frame of the error's StackTrace will
//           be printed in detail.
//
// Retrieving the stack trace of an error or wrapper
//
// New, Errorf, Annotate, and Annotatef record a stack trace at the point they are invoked.
// This information can be retrieved with the StackTracer interface that returns
// a StackTrace. Where errors.StackTrace is defined as
//
//     type StackTrace []Frame
//
// The Frame type represents a call site in the stack trace. Frame supports
// the fmt.Formatter interface that can be used for printing information about
// the stack trace of this error. For example:
//
//     if stacked := errors.GetStackTracer(err); stacked != nil {
//             for _, f := range stacked.StackTrace() {
//                     fmt.Printf("%+s:%d", f)
//             }
//     }
//
// See the documentation for Frame.Format for more details.
//
// errors.Find can be used to search for an error in the error chain.
package errors

import (
	"fmt"
	"io"
)

// New returns an error with the supplied message.
// New also records the stack trace at the point it was called.
func New(message string) error {
	return &fundamental{
		msg:   message,
		stack: callers(),
	}
}

// Errorf formats according to a format specifier and returns the string
// as a value that satisfies error.
// Errorf also records the stack trace at the point it was called.
func Errorf(format string, args ...interface{}) error {
	return &fundamental{
		msg:   fmt.Sprintf(format, args...),
		stack: callers(),
	}
}

// StackTraceAware is an optimization to avoid repetitive traversals of an error chain.
// HasStack checks for this marker first.
// Annotate/Wrap and Annotatef/Wrapf will produce this marker.
type StackTraceAware interface {
	HasStack() bool
}

// HasStack tells whether a StackTracer exists in the error chain
func HasStack(err error) bool {
	if errWithStack, ok := err.(StackTraceAware); ok {
		return errWithStack.HasStack()
	}
	return GetStackTracer(err) != nil
}

// fundamental is an error that has a message and a stack, but no caller.
type fundamental struct {
	msg string
	*stack
}

func (f *fundamental) Error() string { return f.msg }

func (f *fundamental) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			io.WriteString(s, f.msg)
			f.stack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, f.msg)
	case 'q':
		fmt.Fprintf(s, "%q", f.msg)
	}
}

// WithStack annotates err with a stack trace at the point WithStack was called.
// If err is nil, WithStack returns nil.
//
// For most use cases this is deprecated and AddStack should be used (which will ensure just one stack trace).
// However, one may want to use this in some situations, for example to create a 2nd trace across a goroutine.
func WithStack(err error) error {
	if err == nil {
		return nil
	}

	return &withStack{
		err,
		callers(),
	}
}

// AddStack is similar to WithStack.
// However, it will first check with HasStack to see if a stack trace already exists in the causer chain before creating another one.
func AddStack(err error) error {
	if HasStack(err) {
		return err
	}
	return WithStack(err)
}

type withStack struct {
	error
	*stack
}

func (w *withStack) Cause() error { return w.error }

func (w *withStack) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v", w.Cause())
			w.stack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, w.Error())
	case 'q':
		fmt.Fprintf(s, "%q", w.Error())
	}
}

// Wrap returns an error annotating err with a stack trace
// at the point Wrap is called, and the supplied message.
// If err is nil, Wrap returns nil.
//
// For most use cases this is deprecated in favor of Annotate.
// Annotate avoids creating duplicate stack traces.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	hasStack := HasStack(err)
	err = &withMessage{
		cause:         err,
		msg:           message,
		causeHasStack: hasStack,
	}
	return &withStack{
		err,
		callers(),
	}
}

// Wrapf returns an error annotating err with a stack trace
// at the point Wrapf is call, and the format specifier.
// If err is nil, Wrapf returns nil.
//
// For most use cases this is deprecated in favor of Annotatef.
// Annotatef avoids creating duplicate stack traces.
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	hasStack := HasStack(err)
	err = &withMessage{
		cause:         err,
		msg:           fmt.Sprintf(format, args...),
		causeHasStack: hasStack,
	}
	return &withStack{
		err,
		callers(),
	}
}

// WithMessage annotates err with a new message.
// If err is nil, WithMessage returns nil.
func WithMessage(err error, message string) error {
	if err == nil {
		return nil
	}
	return &withMessage{
		cause:         err,
		msg:           message,
		causeHasStack: HasStack(err),
	}
}

type withMessage struct {
	cause         error
	msg           string
	causeHasStack bool
}

func (w *withMessage) Error() string  { return w.msg + ": " + w.cause.Error() }
func (w *withMessage) Cause() error   { return w.cause }
func (w *withMessage) HasStack() bool { return w.causeHasStack }

func (w *withMessage) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v\n", w.Cause())
			io.WriteString(s, w.msg)
			return
		}
		fallthrough
	case 's', 'q':
		io.WriteString(s, w.Error())
	}
}

// Cause returns the underlying cause of the error, if possible.
// An error value has a cause if it implements the following
// interface:
//
//     type causer interface {
//            Cause() error
//     }
//
// If the error does not implement Cause, the original error will
// be returned. If the error is nil, nil will be returned without further
// investigation.
func Cause(err error) error {
	cause := Unwrap(err)
	if cause == nil {
		return err
	}
	return Cause(cause)
}

// Unwrap uses causer to return the next error in the chain or nil.
// This goes one-level deeper, whereas Cause goes as far as possible
func Unwrap(err error) error {
	type causer interface {
		Cause() error
	}
	if unErr, ok := err.(causer); ok {
		return unErr.Cause()
	}
	return nil
}

// Find an error in the chain that matches a test function.
// returns nil if no error is found.
func Find(origErr error, test func(error) bool) error {
	var foundErr error
	WalkDeep(origErr, func(err error) bool {
		if test(err) {
			foundErr = err
			return true
		}
		return false
	})
	return foundErr
}

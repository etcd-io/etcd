package e

import (
	"errors"
	"slices"
	"strings"

	"dev.gaijin.team/go/golib/fields"
)

// Err represents a custom error type that supports error chaining and structured metadata fields.
type Err struct {
	errs   []error
	fields fields.List
}

// New returns a new Err with the given reason and optional fields.
// The returned error can be further wrapped or annotated with additional fields.
func New(reason string, f ...fields.Field) *Err {
	return From(errors.New(reason), f...) //nolint:err113
}

// NewFrom returns a new Err with the given reason, wrapping the provided error, and optional fields.
// If wrapped is nil, it behaves like New.
func NewFrom(reason string, wrapped error, f ...fields.Field) *Err {
	if wrapped == nil {
		return New(reason, f...)
	}

	return &Err{
		errs:   []error{errors.New(reason), wrapped}, //nolint:err113
		fields: f,
	}
}

// From converts any error to an Err, optionally adding fields. This is not true wrapping;
// unwrapping will not return the original error. Passing nil results in an Err with reason "error(nil)".
func From(origin error, f ...fields.Field) *Err {
	if origin == nil {
		origin = errors.New("error(nil)") //nolint:err113
	}

	return &Err{
		errs:   []error{origin},
		fields: f,
	}
}

// Wrap returns a new Err that wraps the provided error with the current Err as context.
// Additional fields can be attached. If err is nil, it is replaced with an Err for "error(nil)".
func (e *Err) Wrap(err error, f ...fields.Field) *Err {
	if err == nil {
		err = errors.New("error(nil)") //nolint:err113
	}

	return &Err{
		errs:   []error{e, err},
		fields: f,
	}
}

// Error returns the string representation of the Err, including reason, fields, and wrapped errors.
func (e *Err) Error() string {
	b := &strings.Builder{}
	writeTo(b, e)

	return b.String()
}

func writeTo(b *strings.Builder, err error) {
	if b.Len() > 0 {
		b.WriteString(": ")
	}

	ee, ok := err.(*Err) //nolint:errorlint
	if !ok {
		b.WriteString(err.Error())

		return
	}

	b.WriteString(ee.Reason())

	if ee == nil {
		return
	}

	if len(ee.fields) > 0 {
		b.WriteRune(' ')
		ee.fields.WriteTo(b)
	}

	if len(ee.errs) > 1 {
		writeTo(b, ee.errs[1])
	}
}

// Clone returns a new Err with the same error, wrapped error, and a cloned fields container.
func (e *Err) Clone() *Err {
	return &Err{
		errs:   slices.Clone(e.errs),
		fields: slices.Clone(e.fields),
	}
}

// WithFields returns a new Err with the same error and the provided fields.
func (e *Err) WithFields(f ...fields.Field) *Err {
	return From(e, f...)
}

// WithField returns a new Err with the same error and a single additional field.
func (e *Err) WithField(key string, val any) *Err {
	return e.WithFields(fields.F(key, val))
}

// Fields returns the metadata fields attached to the Err.
func (e *Err) Fields() fields.List {
	return e.fields
}

// Reason returns the reason string of the Err, without fields or wrapped errors.
// If the Err is nil, returns "(*e.Err)(nil)". If empty, returns "(*e.Err)(empty)".
func (e *Err) Reason() string {
	if e == nil {
		return "(*e.Err)(nil)"
	}

	if len(e.errs) == 0 {
		return "(*e.Err)(empty)"
	}

	return e.errs[0].Error()
}

// Unwrap returns the underlying errors for compatibility with errors.Is and errors.As.
//
// Deprecated: This method is for internal use only. Prefer using the errors package directly.
func (e *Err) Unwrap() []error {
	return e.errs
}

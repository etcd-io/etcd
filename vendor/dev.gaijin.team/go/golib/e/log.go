package e

import (
	"dev.gaijin.team/go/golib/fields"
)

// ErrorLogger defines a function that logs an error message, an error, and optional fields.
type ErrorLogger func(msg string, err error, fs ...fields.Field)

// Log logs the provided error using the given ErrorLogger function.
//
// If err is nil, Log does nothing. If err is of type Err, its reason is used as the log message,
// the wrapped error is passed as the error, and its fields are passed as log fields.
// For other error types, err.Error() is used as the message and nil is passed as the error.
func Log(err error, f ErrorLogger) {
	if err == nil {
		return
	}

	// We're not interested in wrapped error, therefore we're only typecasting it.
	if e, ok := err.(*Err); ok { //nolint:errorlint
		var wrapped error
		if len(e.errs) > 1 {
			wrapped = e.errs[1]
		}

		f(e.Reason(), wrapped, e.fields...)

		return
	}

	f(err.Error(), nil)
}

// Package e provides a custom error type with support for error chaining and
// structured metadata fields.
//
// The main type, Err, enables convenient error wrapping and the attachment of
// key-value metadata fields (fields.Field). Errors can be wrapped to add context
// and enriched with fields for structured logging or diagnostics.
//
// Example: Wrapping errors with context
//
//	var ErrJSONParseFailed = e.New("JSON parse failed")
//	var val any
//	if err := json.Unmarshal([]byte(`["invalid", "json]`), &val); err != nil {
//		return ErrJSONParseFailed.Wrap(err)
//		// Output: "JSON parse failed: unexpected end of JSON input"
//	}
//
// Example: Enriching errors with fields
//
//	err := e.New("operation failed").WithField("user_id", 42)
//	// Output: "operation failed (user_id=42)"
//
//	err = err.Wrap(e.New("db error").WithFields(fields.F("query", "SELECT *")))
//	// Output: "operation failed (user_id=42): db error (query=SELECT *)"
//
// Any error can be converted to an Err using the From function. This does not
// wrap the error; unwrapping will not return the original error.
//
//	e.From(errors.New("error")) // "error"
//	errors.Unwrap(e.From(errors.New("error"))) // nil
//
// The Err string format is:
//
//	<reason> (fields...): <wrapped error string>
//
// Fields are always enclosed in parentheses, and wrapped errors are separated by
// a colon and space.
//
// All methods that return errors create new instances; errors are immutable.
//
// Deprecated methods Unwrap, Is, and As are present for compatibility with the
// errors package, but should not be used directly. Use the errors package
// functions instead.
//
// The Log function logs errors using our logger abstraction - logger.Logger,
// extracting reason, wrapped error, and fields, logging them appropriately.
package e

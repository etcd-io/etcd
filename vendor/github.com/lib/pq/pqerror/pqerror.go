//go:generate go run gen.go

// Package pqerror contains PostgreSQL error codes for use with pq.Error.
package pqerror

// Code is a five-character error code.
type Code string

// Name returns a more human friendly rendering of the error code, namely the
// "condition name".
func (ec Code) Name() string { return errorCodeNames[ec] }

// Class returns the error class, e.g. "28".
func (ec Code) Class() Class { return Class(ec[:2]) }

// Class is only the class part of an error code.
type Class string

// Name returns the condition name of an error class.  It is equivalent to the
// condition name of the "standard" error code (i.e. the one having the last
// three characters "000").
func (ec Class) Name() string { return errorCodeNames[Code(ec+"000")] }

// TODO(v2): use "type Severity string" for the below.

// Error severity values.
const (
	SeverityFatal   = "FATAL"
	SeverityPanic   = "PANIC"
	SeverityWarning = "WARNING"
	SeverityNotice  = "NOTICE"
	SeverityDebug   = "DEBUG"
	SeverityInfo    = "INFO"
	SeverityLog     = "LOG"
)

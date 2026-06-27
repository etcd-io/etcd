package pq

import (
	"bytes"
	"database/sql"

	"github.com/lib/pq/pqerror"
)

// [pq.Error.Severity] values.
//
// Deprecated: use pqerror.Severity[..] values.
//
//go:fix inline
const (
	Efatal   = pqerror.SeverityFatal
	Epanic   = pqerror.SeverityPanic
	Ewarning = pqerror.SeverityWarning
	Enotice  = pqerror.SeverityNotice
	Edebug   = pqerror.SeverityDebug
	Einfo    = pqerror.SeverityInfo
	Elog     = pqerror.SeverityLog
)

// PGError is an interface used by previous versions of pq.
//
// Deprecated: use the Error type. This is never used.
type PGError interface {
	Error() string
	Fatal() bool
	Get(k byte) (v string)
}

// Get implements the legacy PGError interface.
//
// Deprecated: new code should use the fields of the Error struct directly.
func (e *Error) Get(k byte) (v string) {
	switch k {
	case 'S':
		return e.Severity
	case 'C':
		return string(e.Code)
	case 'M':
		return e.Message
	case 'D':
		return e.Detail
	case 'H':
		return e.Hint
	case 'P':
		return e.Position
	case 'p':
		return e.InternalPosition
	case 'q':
		return e.InternalQuery
	case 'W':
		return e.Where
	case 's':
		return e.Schema
	case 't':
		return e.Table
	case 'c':
		return e.Column
	case 'd':
		return e.DataTypeName
	case 'n':
		return e.Constraint
	case 'F':
		return e.File
	case 'L':
		return e.Line
	case 'R':
		return e.Routine
	}
	return ""
}

// ParseURL converts a url to a connection string for driver.Open.
//
// Deprecated: directly passing an URL to sql.Open("postgres", "postgres://...")
// now works, and calling this manually is no longer required.
func ParseURL(url string) (string, error) { return convertURL(url) }

// NullTime represents a [time.Time] that may be null.
//
// Deprecated: this is an alias for [sql.NullTime].
//
//go:fix inline
type NullTime = sql.NullTime

// CopyIn creates a COPY FROM statement which can be prepared with Tx.Prepare().
// The target table should be visible in search_path.
//
// It copies all columns if the list of columns is empty.
//
// Deprecated: there is no need to use this query builder, you can use:
//
//	tx.Prepare("copy tbl (col1, col2) from stdin")
func CopyIn(table string, columns ...string) string {
	b := bytes.NewBufferString("COPY ")
	BufferQuoteIdentifier(table, b)
	makeStmt(b, columns...)
	return b.String()
}

// CopyInSchema creates a COPY FROM statement which can be prepared with
// Tx.Prepare().
//
// Deprecated: there is no need to use this query builder, you can use:
//
//	tx.Prepare("copy schema.tbl (col1, col2) from stdin")
func CopyInSchema(schema, table string, columns ...string) string {
	b := bytes.NewBufferString("COPY ")
	BufferQuoteIdentifier(schema, b)
	b.WriteRune('.')
	BufferQuoteIdentifier(table, b)
	makeStmt(b, columns...)
	return b.String()
}

func makeStmt(b *bytes.Buffer, columns ...string) {
	if len(columns) == 0 {
		b.WriteString(" FROM STDIN")
		return
	}
	b.WriteString(" (")
	for i, col := range columns {
		if i != 0 {
			b.WriteString(", ")
		}
		BufferQuoteIdentifier(col, b)
	}
	b.WriteString(") FROM STDIN")
}

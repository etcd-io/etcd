package pq

import (
	"database/sql/driver"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lib/pq/pqerror"
)

// Error returned by the PostgreSQL server.
//
// The [Error] method returns the error message and error code:
//
//	pq: invalid input syntax for type json (22P02)
//
// The [ErrorWithDetail] method also includes the error Detail, Hint, and
// location context (if any):
//
//	ERROR:   invalid input syntax for type json (22P02)
//	DETAIL:  Token "asd" is invalid.
//	CONTEXT: line 5, column 8:
//
//	 3 | 'def',
//	 4 | 123,
//	 5 | 'foo', 'asd'::jsonb
//	            ^
type Error struct {
	// [Efatal], [Epanic], [Ewarning], [Enotice], [Edebug], [Einfo], or [Elog].
	// Always present.
	Severity string

	// SQLSTATE code. Always present.
	Code pqerror.Code

	// Primary human-readable error message. This should be accurate but terse
	// (typically one line). Always present.
	Message string

	// Optional secondary error message carrying more detail about the problem.
	// Might run to multiple lines.
	Detail string

	// Optional suggestion what to do about the problem. This is intended to
	// differ from Detail in that it offers advice (potentially inappropriate)
	// rather than hard facts. Might run to multiple lines.
	Hint string

	// error position as an index into the original query string, as decimal
	// ASCII integer. The first character has index 1, and positions are
	// measured in characters not bytes.
	Position string

	// This is defined the same as the Position field, but it is used when the
	// cursor position refers to an internally generated command rather than the
	// one submitted by the client. The InternalQuery field will always appear
	// when this field appears.
	InternalPosition string

	// Text of a failed internally-generated command. This could be, for
	// example, an SQL query issued by a PL/pgSQL function.
	InternalQuery string

	// An indication of the context in which the error occurred. Presently this
	// includes a call stack traceback of active procedural language functions
	// and internally-generated queries. The trace is one entry per line, most
	// recent first.
	Where string

	// If the error was associated with a specific database object, the name of
	// the schema containing that object, if any.
	Schema string

	// If the error was associated with a specific table, the name of the table.
	// (Refer to the schema name field for the name of the table's schema.)
	Table string

	// If the error was associated with a specific table column, the name of the
	// column. (Refer to the schema and table name fields to identify the
	// table.)
	Column string

	// If the error was associated with a specific data type, the name of the
	// data type. (Refer to the schema name field for the name of the data
	// type's schema.)
	DataTypeName string

	// If the error was associated with a specific constraint, the name of the
	// constraint. Refer to fields listed above for the associated table or
	// domain. (For this purpose, indexes are treated as constraints, even if
	// they weren't created with constraint syntax.)
	Constraint string

	// File name of the source-code location where the error was reported.
	File string

	// Line number of the source-code location where the error was reported.
	Line string

	// Name of the source-code routine reporting the error.
	Routine string

	query string
}

type (
	// ErrorCode is a five-character error code.
	//
	// Deprecated: use pqerror.Code
	//
	//go:fix inline
	ErrorCode = pqerror.Code

	// ErrorClass is only the class part of an error code.
	//
	// Deprecated: use pqerror.Class
	//
	//go:fix inline
	ErrorClass = pqerror.Class
)

func parseError(r *readBuf, q string) *Error {
	err := &Error{query: q}
	for t := r.byte(); t != 0; t = r.byte() {
		msg := r.string()
		switch t {
		case 'S':
			err.Severity = msg
		case 'C':
			err.Code = pqerror.Code(msg)
		case 'M':
			err.Message = msg
		case 'D':
			err.Detail = msg
		case 'H':
			err.Hint = msg
		case 'P':
			err.Position = msg
		case 'p':
			err.InternalPosition = msg
		case 'q':
			err.InternalQuery = msg
		case 'W':
			err.Where = msg
		case 's':
			err.Schema = msg
		case 't':
			err.Table = msg
		case 'c':
			err.Column = msg
		case 'd':
			err.DataTypeName = msg
		case 'n':
			err.Constraint = msg
		case 'F':
			err.File = msg
		case 'L':
			err.Line = msg
		case 'R':
			err.Routine = msg
		}
	}
	return err
}

// Fatal returns true if the Error Severity is fatal.
func (e *Error) Fatal() bool { return e.Severity == pqerror.SeverityFatal }

// SQLState returns the SQLState of the error.
func (e *Error) SQLState() string { return string(e.Code) }

func (e *Error) Error() string {
	msg := e.Message
	if e.query != "" && e.Position != "" {
		pos, err := strconv.Atoi(e.Position)
		if err == nil {
			lines := strings.Split(e.query, "\n")
			line, col := posToLine(pos, lines)
			if len(lines) == 1 {
				msg += " at column " + strconv.Itoa(col)
			} else {
				msg += " at position " + strconv.Itoa(line) + ":" + strconv.Itoa(col)
			}
		}
	}

	if e.Code != "" {
		return "pq: " + msg + " (" + string(e.Code) + ")"
	}
	return "pq: " + msg
}

// ErrorWithDetail returns the error message with detailed information and
// location context (if any).
//
// See the documentation on [Error].
func (e *Error) ErrorWithDetail() string {
	b := new(strings.Builder)
	b.Grow(len(e.Message) + len(e.Detail) + len(e.Hint) + 30)
	b.WriteString("ERROR:   ")
	b.WriteString(e.Message)
	if e.Code != "" {
		b.WriteString(" (")
		b.WriteString(string(e.Code))
		b.WriteByte(')')
	}
	if e.Detail != "" {
		b.WriteString("\nDETAIL:  ")
		b.WriteString(e.Detail)
	}
	if e.Hint != "" {
		b.WriteString("\nHINT:    ")
		b.WriteString(e.Hint)
	}

	if e.query != "" && e.Position != "" {
		b.Grow(512)
		pos, err := strconv.Atoi(e.Position)
		if err != nil {
			return b.String()
		}
		lines := strings.Split(e.query, "\n")
		line, col := posToLine(pos, lines)

		fmt.Fprintf(b, "\nCONTEXT: line %d, column %d:\n\n", line, col)
		if line > 2 {
			fmt.Fprintf(b, "% 7d | %s\n", line-2, expandTab(lines[line-3]))
		}
		if line > 1 {
			fmt.Fprintf(b, "% 7d | %s\n", line-1, expandTab(lines[line-2]))
		}
		/// Expand tabs, so that the ^ is at at the correct position, but leave
		/// "column 10-13" intact. Adjusting this to the visual column would be
		/// better, but we don't know the tabsize of the user in their editor,
		/// which can be 8, 4, 2, or something else. We can't know. So leaving
		/// it as the character index is probably the "most correct".
		expanded := expandTab(lines[line-1])
		diff := len(expanded) - len(lines[line-1])
		fmt.Fprintf(b, "% 7d | %s\n", line, expanded)
		fmt.Fprintf(b, "% 10s%s%s\n", "", strings.Repeat(" ", col-1+diff), "^")
	}

	return b.String()
}

func posToLine(pos int, lines []string) (line, col int) {
	read := 0
	for i := range lines {
		line++
		ll := utf8.RuneCountInString(lines[i]) + 1 // +1 for the removed newline
		if read+ll >= pos {
			col = max(pos-read, 1) // Should be lower than 1, but just in case.
			break
		}
		read += ll
	}
	return line, col
}

func expandTab(s string) string {
	var (
		b    strings.Builder
		l    int
		fill = func(n int) string {
			b := make([]byte, n)
			for i := range b {
				b[i] = ' '
			}
			return string(b)
		}
	)
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\t':
			tw := 8 - l%8
			b.WriteString(fill(tw))
			l += tw
		default:
			b.WriteRune(r)
			l += 1
		}
	}
	return b.String()
}

func (cn *conn) handleError(reported error, query ...string) error {
	switch err := reported.(type) {
	case nil:
		return nil
	case runtime.Error, *net.OpError:
		cn.err.set(driver.ErrBadConn)
	case *safeRetryError:
		cn.err.set(driver.ErrBadConn)
		reported = driver.ErrBadConn
	case *Error:
		if len(query) > 0 && query[0] != "" {
			err.query = query[0]
			reported = err
		}
		if err.Fatal() {
			reported = driver.ErrBadConn
		}
	case error:
		if err == io.EOF || err == io.ErrUnexpectedEOF || err.Error() == "remote error: handshake failure" {
			reported = driver.ErrBadConn
		}
	default:
		cn.err.set(driver.ErrBadConn)
		reported = fmt.Errorf("pq: unknown error %T: %[1]s", err)
	}

	// Any time we return ErrBadConn, we need to remember it since *Tx doesn't
	// mark the connection bad in database/sql.
	if reported == driver.ErrBadConn {
		cn.err.set(driver.ErrBadConn)
	}
	return reported
}

package pq

import (
	"database/sql/driver"
	"fmt"
	"io"
	"math"
	"reflect"
	"time"

	"github.com/lib/pq/internal/proto"
	"github.com/lib/pq/oid"
)

type noRows struct{}

var emptyRows noRows

var _ driver.Result = noRows{}

func (noRows) LastInsertId() (int64, error) { return 0, errNoLastInsertID }
func (noRows) RowsAffected() (int64, error) { return 0, errNoRowsAffected }

type (
	rowsHeader struct {
		colNames []string
		colTyps  []fieldDesc
		colFmts  []format
	}
	rows struct {
		cn     *conn
		finish func()
		rowsHeader
		done   bool
		rb     readBuf
		result driver.Result
		tag    string

		next *rowsHeader
	}
)

func (rs *rows) Close() error {
	if finish := rs.finish; finish != nil {
		defer finish()
	}
	// no need to look at cn.bad as Next() will
	for {
		err := rs.Next(nil)
		switch err {
		case nil:
		case io.EOF:
			// rs.Next can return io.EOF on both ReadyForQuery and
			// RowDescription (used with HasNextResultSet). We need to fetch
			// messages until we hit a ReadyForQuery, which is done by waiting
			// for done to be set.
			if rs.done {
				return nil
			}
		default:
			return err
		}
	}
}

func (rs *rows) Columns() []string {
	return rs.colNames
}

func (rs *rows) Result() driver.Result {
	if rs.result == nil {
		return emptyRows
	}
	return rs.result
}

func (rs *rows) Tag() string {
	return rs.tag
}

func (rs *rows) Next(dest []driver.Value) (resErr error) {
	if rs.done {
		return io.EOF
	}
	if err := rs.cn.err.getForNext(); err != nil {
		return err
	}

	for {
		t, err := rs.cn.recv1Buf(&rs.rb)
		if err != nil {
			return rs.cn.handleError(err)
		}
		switch t {
		case proto.ErrorResponse:
			resErr = parseError(&rs.rb, "")
		case proto.CommandComplete, proto.EmptyQueryResponse:
			if t == proto.CommandComplete {
				rs.result, rs.tag, err = rs.cn.parseComplete(rs.rb.string())
				if err != nil {
					return rs.cn.handleError(err)
				}
			}
			continue
		case proto.ReadyForQuery:
			rs.cn.processReadyForQuery(&rs.rb)
			rs.done = true
			if resErr != nil {
				return rs.cn.handleError(resErr)
			}
			return io.EOF
		case proto.DataRow:
			n := rs.rb.int16()
			if resErr != nil {
				rs.cn.err.set(driver.ErrBadConn)
				return fmt.Errorf("pq: unexpected DataRow after error %s", resErr)
			}
			if n < len(dest) {
				dest = dest[:n]
			}
			for i := range dest {
				l := rs.rb.int32()
				if l == -1 {
					dest[i] = nil
					continue
				}
				dest[i], err = decode(&rs.cn.parameterStatus, rs.rb.next(l), rs.colTyps[i].OID, rs.colFmts[i])
				if err != nil {
					return rs.cn.handleError(err)
				}
			}
			return rs.cn.handleError(resErr)
		case proto.RowDescription:
			next := parsePortalRowDescribe(&rs.rb)
			rs.next = &next
			return io.EOF
		default:
			return fmt.Errorf("pq: unexpected message after execute: %q", t)
		}
	}
}

func (rs *rows) HasNextResultSet() bool {
	hasNext := rs.next != nil && !rs.done
	return hasNext
}

func (rs *rows) NextResultSet() error {
	if rs.next == nil {
		return io.EOF
	}
	rs.rowsHeader = *rs.next
	rs.next = nil
	return nil
}

// ColumnTypeScanType returns the value type that can be used to scan types into.
func (rs *rows) ColumnTypeScanType(index int) reflect.Type {
	return rs.colTyps[index].Type()
}

// ColumnTypeDatabaseTypeName return the database system type name.
func (rs *rows) ColumnTypeDatabaseTypeName(index int) string {
	return rs.colTyps[index].Name()
}

// ColumnTypeLength returns the length of the column type if the column is a
// variable length type. If the column is not a variable length type ok
// should return false.
func (rs *rows) ColumnTypeLength(index int) (length int64, ok bool) {
	return rs.colTyps[index].Length()
}

// ColumnTypePrecisionScale should return the precision and scale for decimal
// types. If not applicable, ok should be false.
func (rs *rows) ColumnTypePrecisionScale(index int) (precision, scale int64, ok bool) {
	return rs.colTyps[index].PrecisionScale()
}

const headerSize = 4

type fieldDesc struct {
	// The object ID of the data type.
	OID oid.Oid
	// The data type size (see pg_type.typlen).
	// Note that negative values denote variable-width types.
	Len int
	// The type modifier (see pg_attribute.atttypmod).
	// The meaning of the modifier is type-specific.
	Mod int
}

func (fd fieldDesc) Type() reflect.Type {
	switch fd.OID {
	case oid.T_int8:
		return reflect.TypeOf(int64(0))
	case oid.T_int4:
		return reflect.TypeOf(int32(0))
	case oid.T_int2:
		return reflect.TypeOf(int16(0))
	case oid.T_float8:
		return reflect.TypeOf(float64(0))
	case oid.T_float4:
		return reflect.TypeOf(float32(0))
	case oid.T_varchar, oid.T_text, oid.T_varbit, oid.T_bit:
		return reflect.TypeOf("")
	case oid.T_bool:
		return reflect.TypeOf(false)
	case oid.T_date, oid.T_time, oid.T_timetz, oid.T_timestamp, oid.T_timestamptz:
		return reflect.TypeOf(time.Time{})
	case oid.T_bytea:
		return reflect.TypeOf([]byte(nil))
	default:
		return reflect.TypeOf(new(any)).Elem()
	}
}

func (fd fieldDesc) Name() string {
	return oid.TypeName[fd.OID]
}

func (fd fieldDesc) Length() (length int64, ok bool) {
	switch fd.OID {
	case oid.T_text, oid.T_bytea:
		return math.MaxInt64, true
	case oid.T_varchar, oid.T_bpchar:
		return int64(fd.Mod - headerSize), true
	case oid.T_varbit, oid.T_bit:
		return int64(fd.Mod), true
	default:
		return 0, false
	}
}

func (fd fieldDesc) PrecisionScale() (precision, scale int64, ok bool) {
	switch fd.OID {
	case oid.T_numeric, oid.T__numeric:
		mod := fd.Mod - headerSize
		precision = int64((mod >> 16) & 0xffff)
		scale = int64(mod & 0xffff)
		return precision, scale, true
	default:
		return 0, 0, false
	}
}

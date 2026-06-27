package pq

import (
	"context"
	"database/sql/driver"
	"fmt"
	"os"

	"github.com/lib/pq/internal/proto"
	"github.com/lib/pq/oid"
)

type stmt struct {
	cn   *conn
	name string
	rowsHeader
	colFmtData []byte
	paramTyps  []oid.Oid
	closed     bool
}

func (st *stmt) Close() error {
	if st.closed {
		return nil
	}
	if err := st.cn.err.get(); err != nil {
		return err
	}

	w := st.cn.writeBuf(proto.Close)
	w.byte(proto.Sync)
	w.string(st.name)
	err := st.cn.send(w)
	if err != nil {
		return st.cn.handleError(err)
	}
	err = st.cn.send(st.cn.writeBuf(proto.Sync))
	if err != nil {
		return st.cn.handleError(err)
	}

	t, _, err := st.cn.recv1()
	if err != nil {
		return st.cn.handleError(err)
	}
	if t != proto.CloseComplete {
		st.cn.err.set(driver.ErrBadConn)
		return fmt.Errorf("pq: unexpected close response: %q", t)
	}
	st.closed = true

	t, r, err := st.cn.recv1()
	if err != nil {
		return st.cn.handleError(err)
	}
	if t != proto.ReadyForQuery {
		st.cn.err.set(driver.ErrBadConn)
		return fmt.Errorf("pq: expected ready for query, but got: %q", t)
	}
	st.cn.processReadyForQuery(r)

	return nil
}

func (st *stmt) Query(v []driver.Value) (r driver.Rows, err error) {
	return st.query(toNamedValue(v))
}

func (st *stmt) query(v []driver.NamedValue) (*rows, error) {
	if err := st.cn.err.get(); err != nil {
		return nil, err
	}

	err := st.exec(v)
	if err != nil {
		return nil, st.cn.handleError(err)
	}
	return &rows{
		cn:         st.cn,
		rowsHeader: st.rowsHeader,
	}, nil
}

func (st *stmt) Exec(v []driver.Value) (driver.Result, error) {
	return st.ExecContext(context.Background(), toNamedValue(v))
}

func (st *stmt) exec(v []driver.NamedValue) error {
	if debugProto {
		fmt.Fprintf(os.Stderr, "         START stmt.exec\n")
		defer fmt.Fprintf(os.Stderr, "         END stmt.exec\n")
	}
	if len(v) >= 65536 {
		return fmt.Errorf("pq: got %d parameters but PostgreSQL only supports 65535 parameters", len(v))
	}
	if len(v) != len(st.paramTyps) {
		return fmt.Errorf("pq: got %d parameters but the statement requires %d", len(v), len(st.paramTyps))
	}

	cn := st.cn
	w := cn.writeBuf(proto.Bind)
	w.byte(0) // unnamed portal
	w.string(st.name)

	if cn.cfg.BinaryParameters {
		err := cn.sendBinaryParameters(w, v)
		if err != nil {
			return err
		}
	} else {
		w.int16(0)
		w.int16(len(v))
		for i, x := range v {
			if x.Value == nil {
				w.int32(-1)
			} else {
				b, err := encode(x.Value, st.paramTyps[i])
				if err != nil {
					return err
				}
				if b == nil {
					w.int32(-1)
				} else {
					w.int32(len(b))
					w.bytes(b)
				}
			}
		}
	}
	w.bytes(st.colFmtData)

	w.next(proto.Execute)
	w.byte(0)
	w.int32(0)

	w.next(proto.Sync)
	err := cn.send(w)
	if err != nil {
		return err
	}
	err = cn.readBindResponse()
	if err != nil {
		return err
	}
	return cn.postExecuteWorkaround()
}

func (st *stmt) NumInput() int {
	return len(st.paramTyps)
}

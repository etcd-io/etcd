package pq

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"time"

	"github.com/lib/pq/internal/proto"
)

const watchCancelDialContextTimeout = 10 * time.Second

// Implement the "QueryerContext" interface
func (cn *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	finish := cn.watchCancel(ctx)
	r, err := cn.query(query, args)
	if err != nil {
		if finish != nil {
			finish()
		}
		return nil, err
	}
	r.finish = finish
	return r, nil
}

// Implement the "ExecerContext" interface
func (cn *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	list := make([]driver.Value, len(args))
	for i, nv := range args {
		list[i] = nv.Value
	}

	if finish := cn.watchCancel(ctx); finish != nil {
		defer finish()
	}

	return cn.Exec(query, list)
}

// Implement the "ConnPrepareContext" interface
func (cn *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if finish := cn.watchCancel(ctx); finish != nil {
		defer finish()
	}
	return cn.Prepare(query)
}

// Implement the "ConnBeginTx" interface
func (cn *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	var mode string
	switch sql.IsolationLevel(opts.Isolation) {
	case sql.LevelDefault:
		// Don't touch mode: use the server's default
	case sql.LevelReadUncommitted:
		mode = " ISOLATION LEVEL READ UNCOMMITTED"
	case sql.LevelReadCommitted:
		mode = " ISOLATION LEVEL READ COMMITTED"
	case sql.LevelRepeatableRead:
		mode = " ISOLATION LEVEL REPEATABLE READ"
	case sql.LevelSerializable:
		mode = " ISOLATION LEVEL SERIALIZABLE"
	default:
		return nil, fmt.Errorf("pq: isolation level not supported: %d", opts.Isolation)
	}
	if opts.ReadOnly {
		mode += " READ ONLY"
	} else {
		mode += " READ WRITE"
	}

	tx, err := cn.begin(mode)
	if err != nil {
		return nil, err
	}
	cn.txnFinish = cn.watchCancel(ctx)
	return tx, nil
}

func (cn *conn) Ping(ctx context.Context) error {
	if finish := cn.watchCancel(ctx); finish != nil {
		defer finish()
	}
	rows, err := cn.simpleQuery(";")
	if err != nil {
		return driver.ErrBadConn
	}
	_ = rows.Close()
	return nil
}

func (cn *conn) watchCancel(ctx context.Context) func() {
	if done := ctx.Done(); done != nil {
		finished := make(chan struct{}, 1)
		go func() {
			select {
			case <-done:
				select {
				case finished <- struct{}{}:
				default:
					// We raced with the finish func, let the next query handle this with the
					// context.
					return
				}

				// Set the connection state to bad so it does not get reused.
				cn.err.set(ctx.Err())

				// At this point the function level context is canceled,
				// so it must not be used for the additional network
				// request to cancel the query.
				// Create a new context to pass into the dial.
				ctxCancel, cancel := context.WithTimeout(context.Background(), watchCancelDialContextTimeout)
				defer cancel()

				_ = cn.cancel(ctxCancel)
			case <-finished:
			}
		}()
		return func() {
			select {
			case <-finished:
				cn.err.set(ctx.Err())
				_ = cn.Close()
			case finished <- struct{}{}:
			}
		}
	}
	return nil
}

func (cn *conn) cancel(ctx context.Context) error {
	// Use a copy since a new connection is created here. This is necessary
	// because cancel is called from a goroutine in watchCancel.
	cfg := cn.cfg.Clone()

	c, err := dial(ctx, cn.dialer, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	cn2 := conn{c: c}
	err = cn2.ssl(cfg, cfg.SSLMode)
	if err != nil {
		return err
	}

	w := cn2.writeBuf(0)
	w.int32(proto.CancelRequestCode)
	w.int32(cn.pid)
	w.bytes(cn.secretKey)
	if err := cn2.sendStartupPacket(w); err != nil {
		return err
	}

	// Read until EOF to ensure that the server received the cancel.
	_, err = io.Copy(io.Discard, c)
	return err
}

// Implement the "StmtQueryContext" interface
func (st *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	finish := st.watchCancel(ctx)
	r, err := st.query(args)
	if err != nil {
		if finish != nil {
			finish()
		}
		return nil, err
	}
	r.finish = finish
	return r, nil
}

// Implement the "StmtExecContext" interface
func (st *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if finish := st.watchCancel(ctx); finish != nil {
		defer finish()
	}
	if err := st.cn.err.get(); err != nil {
		return nil, err
	}

	err := st.exec(args)
	if err != nil {
		return nil, st.cn.handleError(err)
	}
	res, _, err := st.cn.readExecuteResponse("simple query")
	return res, st.cn.handleError(err)
}

// watchCancel is implemented on stmt in order to not mark the parent conn as bad
func (st *stmt) watchCancel(ctx context.Context) func() {
	if done := ctx.Done(); done != nil {
		finished := make(chan struct{})
		go func() {
			select {
			case <-done:
				// At this point the function level context is canceled, so it
				// must not be used for the additional network request to cancel
				// the query. Create a new context to pass into the dial.
				ctxCancel, cancel := context.WithTimeout(context.Background(), watchCancelDialContextTimeout)
				defer cancel()

				_ = st.cancel(ctxCancel)
				finished <- struct{}{}
			case <-finished:
			}
		}()
		return func() {
			select {
			case <-finished:
			case finished <- struct{}{}:
			}
		}
	}
	return nil
}

func (st *stmt) cancel(ctx context.Context) error {
	return st.cn.cancel(ctx)
}

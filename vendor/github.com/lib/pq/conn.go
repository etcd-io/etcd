package pq

import (
	"bufio"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lib/pq/internal/pgpass"
	"github.com/lib/pq/internal/pqsql"
	"github.com/lib/pq/internal/pqutil"
	"github.com/lib/pq/internal/proto"
	"github.com/lib/pq/oid"
	"github.com/lib/pq/scram"
)

// Common error types
var (
	ErrNotSupported              = errors.New("pq: unsupported command")
	ErrInFailedTransaction       = errors.New("pq: could not complete operation in a failed transaction")
	ErrSSLNotSupported           = errors.New("pq: SSL is not enabled on the server")
	ErrCouldNotDetectUsername    = errors.New("pq: could not detect default username; please provide one explicitly")
	ErrSSLKeyUnknownOwnership    = pqutil.ErrSSLKeyUnknownOwnership
	ErrSSLKeyHasWorldPermissions = pqutil.ErrSSLKeyHasWorldPermissions

	errQueryInProgress = errors.New("pq: there is already a query being processed on this connection")
	errUnexpectedReady = errors.New("unexpected ReadyForQuery")
	errNoRowsAffected  = errors.New("no RowsAffected available after the empty statement")
	errNoLastInsertID  = errors.New("no LastInsertId available after the empty statement")
)

// Compile time validation that our types implement the expected interfaces
var (
	_ driver.Driver             = Driver{}
	_ driver.ConnBeginTx        = (*conn)(nil)
	_ driver.ConnPrepareContext = (*conn)(nil)
	_ driver.Execer             = (*conn)(nil) //lint:ignore SA1019 x
	_ driver.ExecerContext      = (*conn)(nil)
	_ driver.NamedValueChecker  = (*conn)(nil)
	_ driver.Pinger             = (*conn)(nil)
	_ driver.Queryer            = (*conn)(nil) //lint:ignore SA1019 x
	_ driver.QueryerContext     = (*conn)(nil)
	_ driver.SessionResetter    = (*conn)(nil)
	_ driver.Validator          = (*conn)(nil)
	_ driver.StmtExecContext    = (*stmt)(nil)
	_ driver.StmtQueryContext   = (*stmt)(nil)
)

func init() {
	sql.Register("postgres", &Driver{})
}

var debugProto = func() bool {
	// Check for exactly "1" (rather than mere existence) so we can add
	// options/flags in the future. I don't know if we ever want that, but it's
	// nice to leave the option open.
	return os.Getenv("PQGO_DEBUG") == "1"
}()

// Driver is the Postgres database driver.
type Driver struct{}

// Open opens a new connection to the database. name is a connection string.
// Most users should only use it through database/sql package from the standard
// library.
func (d Driver) Open(name string) (driver.Conn, error) {
	return Open(name)
}

// Parameters sent by PostgreSQL on startup.
type parameterStatus struct {
	serverVersion                            int
	currentLocation                          *time.Location
	inHotStandby, defaultTransactionReadOnly sql.NullBool
}

type format int

const (
	formatText   format = 0
	formatBinary format = 1
)

var (
	// One result-column format code with the value 1 (i.e. all binary).
	colFmtDataAllBinary = []byte{0, 1, 0, 1}

	// No result-column format codes (i.e. all text).
	colFmtDataAllText = []byte{0, 0}
)

type transactionStatus byte

const (
	txnStatusIdle                transactionStatus = 'I'
	txnStatusIdleInTransaction   transactionStatus = 'T'
	txnStatusInFailedTransaction transactionStatus = 'E'
)

func (s transactionStatus) String() string {
	switch s {
	case txnStatusIdle:
		return "idle"
	case txnStatusIdleInTransaction:
		return "idle in transaction"
	case txnStatusInFailedTransaction:
		return "in a failed transaction"
	default:
		panic(fmt.Sprintf("pq: unknown transactionStatus %d", s))
	}
}

// Dialer is the dialer interface. It can be used to obtain more control over
// how pq creates network connections.
type Dialer interface {
	Dial(network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
}

// DialerContext is the context-aware dialer interface.
type DialerContext interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type defaultDialer struct {
	d net.Dialer
}

func (d defaultDialer) Dial(network, address string) (net.Conn, error) {
	return d.d.Dial(network, address)
}

func (d defaultDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return d.DialContext(ctx, network, address)
}

func (d defaultDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.d.DialContext(ctx, network, address)
}

type conn struct {
	c         net.Conn
	buf       *bufio.Reader
	namei     int
	scratch   [512]byte
	txnStatus transactionStatus
	txnFinish func()

	// Save connection arguments to use during CancelRequest.
	dialer          Dialer
	cfg             Config
	parameterStatus parameterStatus

	saveMessageType   proto.ResponseCode
	saveMessageBuffer []byte

	// If an error is set this connection is bad and all public-facing
	// functions should return the appropriate error by calling get()
	// (ErrBadConn) or getForNext().
	err syncErr

	secretKey           []byte              // Cancellation key for CancelRequest messages.
	pid                 int                 // Cancellation PID.
	inProgress          atomic.Bool         // This connection is in the middle of a processing a request.
	noticeHandler       func(*Error)        // If not nil, notices will be synchronously sent here
	notificationHandler func(*Notification) // If not nil, notifications will be synchronously sent here
	gss                 GSS                 // GSSAPI context
}

type syncErr struct {
	err error
	sync.Mutex
}

// Return ErrBadConn if connection is bad.
func (e *syncErr) get() error {
	e.Lock()
	defer e.Unlock()
	if e.err != nil {
		return driver.ErrBadConn
	}
	return nil
}

// Return the error set on the connection. Currently only used by rows.Next.
func (e *syncErr) getForNext() error {
	e.Lock()
	defer e.Unlock()
	return e.err
}

// Set error, only if it isn't set yet.
func (e *syncErr) set(err error) {
	if err == nil {
		panic("attempt to set nil err")
	}
	e.Lock()
	defer e.Unlock()
	if e.err == nil {
		e.err = err
	}
}

func (cn *conn) writeBuf(b proto.RequestCode) *writeBuf {
	cn.scratch[0] = byte(b)
	return &writeBuf{
		buf: cn.scratch[:5],
		pos: 1,
	}
}

// Open opens a new connection to the database. dsn is a connection string. Most
// users should only use it through database/sql package from the standard
// library.
func Open(dsn string) (_ driver.Conn, err error) {
	return DialOpen(defaultDialer{}, dsn)
}

// DialOpen opens a new connection to the database using a dialer.
func DialOpen(d Dialer, dsn string) (_ driver.Conn, err error) {
	c, err := NewConnector(dsn)
	if err != nil {
		return nil, err
	}
	c.Dialer(d)
	return c.open(context.Background())
}

func (c *Connector) open(ctx context.Context) (*conn, error) {
	tsa := c.cfg.TargetSessionAttrs
restartAll:
	var (
		errs []error
		app  = func(err error, cfg Config) bool {
			if err != nil {
				if debugProto {
					fmt.Fprintln(os.Stderr, "CONNECT  (error)", err)
				}
				errs = append(errs, fmt.Errorf("connecting to %s:%d: %w", cfg.Host, cfg.Port, err))
			}
			return err != nil
		}
	)
	for _, cfg := range c.cfg.hosts() {
		mode := cfg.SSLMode
	restartHost:
		if debugProto {
			fmt.Fprintln(os.Stderr, "CONNECT ", cfg.string())
		}

		cfg.SSLMode = mode
		cn := &conn{cfg: cfg, dialer: c.dialer}
		cn.cfg.Password = pgpass.PasswordFromPgpass(cn.cfg.Passfile, cn.cfg.User, cn.cfg.Password,
			cn.cfg.Host, strconv.Itoa(int(cn.cfg.Port)), cn.cfg.Database)

		var err error
		cn.c, err = dial(ctx, c.dialer, cn.cfg)
		if app(err, cfg) {
			continue
		}

		err = cn.ssl(cn.cfg, mode)
		if err != nil && mode == SSLModePrefer {
			mode = SSLModeDisable
			goto restartHost
		}
		if app(err, cfg) {
			if cn.c != nil {
				_ = cn.c.Close()
			}
			continue
		}

		cn.buf = bufio.NewReader(cn.c)
		err = cn.startup(cn.cfg)
		if err != nil && mode == SSLModeAllow {
			mode = SSLModeRequire
			goto restartHost
		}
		if app(err, cfg) {
			_ = cn.c.Close()
			continue
		}

		// Reset the deadline, in case one was set (see dial)
		if cn.cfg.ConnectTimeout > 0 {
			err := cn.c.SetDeadline(time.Time{})
			if app(err, cfg) {
				_ = cn.c.Close()
				continue
			}
		}

		err = cn.checkTSA(tsa)
		if app(err, cfg) {
			_ = cn.c.Close()
			continue
		}

		return cn, nil
	}

	// target_session_attrs=prefer-standby is treated as standby in checkTSA; we
	// ran out of hosts so none are on standby. Clear the setting and try again.
	if c.cfg.TargetSessionAttrs == TargetSessionAttrsPreferStandby {
		tsa = TargetSessionAttrsAny
		goto restartAll
	}

	if len(c.cfg.Multi) == 0 {
		// Remove the "connecting to [..]" when we have just one host, so the
		// error is identical to what we had before.
		return nil, errors.Unwrap(errs[0])
	}
	return nil, fmt.Errorf("pq: could not connect to any of the hosts:\n%w", errors.Join(errs...))
}

func (cn *conn) getBool(query string) (bool, error) {
	res, err := cn.simpleQuery(query)
	if err != nil {
		return false, err
	}
	defer res.Close()

	v := make([]driver.Value, 1)
	err = res.Next(v)
	if err != nil {
		return false, err
	}

	switch vv := v[0].(type) {
	default:
		return false, fmt.Errorf("parseBool: unknown type %T: %[1]v", v[0])
	case bool:
		return vv, nil
	case string:
		vv, ok := v[0].(string)
		if !ok {
			return false, err
		}
		return vv == "on", nil
	}
}

func (cn *conn) checkTSA(tsa TargetSessionAttrs) error {
	var (
		geths = func() (hs bool, err error) {
			hs = cn.parameterStatus.inHotStandby.Bool
			if !cn.parameterStatus.inHotStandby.Valid {
				hs, err = cn.getBool("select pg_catalog.pg_is_in_recovery()")
			}
			return hs, err
		}
		getro = func() (ro bool, err error) {
			ro = cn.parameterStatus.defaultTransactionReadOnly.Bool
			if !cn.parameterStatus.defaultTransactionReadOnly.Valid {
				ro, err = cn.getBool("show transaction_read_only")
			}
			return ro, err
		}
	)

	switch tsa {
	default:
		panic("unreachable")
	case "", TargetSessionAttrsAny:
		return nil
	case TargetSessionAttrsReadWrite, TargetSessionAttrsReadOnly:
		readonly, err := getro()
		if err != nil {
			return err
		}
		if !cn.parameterStatus.defaultTransactionReadOnly.Valid {
			var err error
			readonly, err = cn.getBool("show transaction_read_only")
			if err != nil {
				return err
			}
		}
		switch {
		case tsa == TargetSessionAttrsReadOnly && !readonly:
			return errors.New("session is not read-only")
		case tsa == TargetSessionAttrsReadWrite:
			if readonly {
				return errors.New("session is read-only")
			}
			hs, err := geths()
			if err != nil {
				return err
			}
			if hs {
				return errors.New("server is in hot standby mode")
			}
			return nil
		default:
			return nil
		}
	case TargetSessionAttrsPrimary, TargetSessionAttrsStandby, TargetSessionAttrsPreferStandby:
		hs, err := geths()
		if err != nil {
			return err
		}
		switch {
		case (tsa == TargetSessionAttrsStandby || tsa == TargetSessionAttrsPreferStandby) && !hs:
			return errors.New("server is not in hot standby mode")
		case tsa == TargetSessionAttrsPrimary && hs:
			return errors.New("server is in hot standby mode")
		default:
			return nil
		}
	}
}

func dial(ctx context.Context, d Dialer, cfg Config) (net.Conn, error) {
	network, address := cfg.network()

	// Zero or not specified means wait indefinitely.
	if cfg.ConnectTimeout > 0 {
		// connect_timeout should apply to the entire connection establishment
		// procedure, so we both use a timeout for the TCP connection
		// establishment and set a deadline for doing the initial handshake. The
		// deadline is then reset after startup() is done.
		var (
			deadline = time.Now().Add(cfg.ConnectTimeout)
			conn     net.Conn
			err      error
		)
		if dctx, ok := d.(DialerContext); ok {
			ctx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
			defer cancel()
			conn, err = dctx.DialContext(ctx, network, address)
		} else {
			conn, err = d.DialTimeout(network, address, cfg.ConnectTimeout)
		}
		if err != nil {
			return nil, err
		}
		err = conn.SetDeadline(deadline)
		return conn, err
	}
	if dctx, ok := d.(DialerContext); ok {
		return dctx.DialContext(ctx, network, address)
	}
	return d.Dial(network, address)
}

func (cn *conn) isInTransaction() bool {
	return cn.txnStatus == txnStatusIdleInTransaction ||
		cn.txnStatus == txnStatusInFailedTransaction
}

func (cn *conn) checkIsInTransaction(intxn bool) error {
	if cn.isInTransaction() != intxn {
		cn.err.set(driver.ErrBadConn)
		return fmt.Errorf("pq: unexpected transaction status %v", cn.txnStatus)
	}
	return nil
}

func (cn *conn) Begin() (_ driver.Tx, err error) {
	return cn.begin("")
}

func (cn *conn) begin(mode string) (_ driver.Tx, err error) {
	if err := cn.err.get(); err != nil {
		return nil, err
	}
	if err := cn.checkIsInTransaction(false); err != nil {
		return nil, err
	}

	_, commandTag, err := cn.simpleExec("BEGIN" + mode)
	if err != nil {
		return nil, cn.handleError(err)
	}
	if commandTag != "BEGIN" {
		cn.err.set(driver.ErrBadConn)
		return nil, fmt.Errorf("unexpected command tag %s", commandTag)
	}
	if cn.txnStatus != txnStatusIdleInTransaction {
		cn.err.set(driver.ErrBadConn)
		return nil, fmt.Errorf("unexpected transaction status %v", cn.txnStatus)
	}
	return cn, nil
}

func (cn *conn) closeTxn() {
	if finish := cn.txnFinish; finish != nil {
		finish()
	}
}

func (cn *conn) Commit() error {
	defer cn.closeTxn()
	if err := cn.err.get(); err != nil {
		return err
	}
	if err := cn.checkIsInTransaction(true); err != nil {
		return err
	}

	// We don't want the client to think that everything is okay if it tries
	// to commit a failed transaction.  However, no matter what we return,
	// database/sql will release this connection back into the free connection
	// pool so we have to abort the current transaction here.  Note that you
	// would get the same behaviour if you issued a COMMIT in a failed
	// transaction, so it's also the least surprising thing to do here.
	if cn.txnStatus == txnStatusInFailedTransaction {
		if err := cn.rollback(); err != nil {
			return err
		}
		return ErrInFailedTransaction
	}

	_, commandTag, err := cn.simpleExec("COMMIT")
	if err != nil {
		if cn.isInTransaction() {
			cn.err.set(driver.ErrBadConn)
		}
		return cn.handleError(err)
	}
	if commandTag != "COMMIT" {
		cn.err.set(driver.ErrBadConn)
		return fmt.Errorf("unexpected command tag %s", commandTag)
	}
	return cn.checkIsInTransaction(false)
}

func (cn *conn) Rollback() error {
	defer cn.closeTxn()
	if err := cn.err.get(); err != nil {
		return err
	}

	err := cn.rollback()
	if err != nil {
		return cn.handleError(err)
	}
	return nil
}

func (cn *conn) rollback() (err error) {
	if err := cn.checkIsInTransaction(true); err != nil {
		return err
	}

	_, commandTag, err := cn.simpleExec("ROLLBACK")
	if err != nil {
		if cn.isInTransaction() {
			cn.err.set(driver.ErrBadConn)
		}
		return err
	}
	if commandTag != "ROLLBACK" {
		return fmt.Errorf("unexpected command tag %s", commandTag)
	}
	return cn.checkIsInTransaction(false)
}

func (cn *conn) gname() string {
	cn.namei++
	return strconv.FormatInt(int64(cn.namei), 10)
}

func (cn *conn) simpleExec(q string) (res driver.Result, commandTag string, resErr error) {
	if debugProto {
		fmt.Fprintln(os.Stderr, "         START conn.simpleExec")
		defer fmt.Fprintln(os.Stderr, "         END conn.simpleExec")
	}

	b := cn.writeBuf(proto.Query)
	b.string(q)
	err := cn.send(b)
	if err != nil {
		return nil, "", err
	}

	for {
		t, r, err := cn.recv1()
		if err != nil {
			return nil, "", err
		}
		switch t {
		case proto.CommandComplete:
			res, commandTag, err = cn.parseComplete(r.string())
			if err != nil {
				return nil, "", err
			}
		case proto.ReadyForQuery:
			cn.processReadyForQuery(r)
			if res == nil && resErr == nil {
				resErr = errUnexpectedReady
			}
			return res, commandTag, resErr
		case proto.ErrorResponse:
			resErr = parseError(r, q)
		case proto.EmptyQueryResponse:
			res = emptyRows
		case proto.RowDescription, proto.DataRow:
			// ignore any results
		default:
			cn.err.set(driver.ErrBadConn)
			return nil, "", fmt.Errorf("pq: unknown response for simple query: %q", t)
		}
	}
}

func (cn *conn) simpleQuery(q string) (*rows, error) {
	if debugProto {
		fmt.Fprintln(os.Stderr, "         START conn.simpleQuery")
		defer fmt.Fprintln(os.Stderr, "         END conn.simpleQuery")
	}

	b := cn.writeBuf(proto.Query)
	b.string(q)
	err := cn.send(b)
	if err != nil {
		return nil, cn.handleError(err, q)
	}

	var (
		res    *rows
		resErr error
	)
	for {
		t, r, err := cn.recv1()
		if err != nil {
			return nil, cn.handleError(err, q)
		}
		switch t {
		case proto.CommandComplete, proto.EmptyQueryResponse:
			// We allow queries which don't return any results through Query as
			// well as Exec. We still have to give database/sql a rows object
			// the user can close, though, to avoid connections from being
			// leaked. A "rows" with done=true works fine for that purpose.
			if resErr != nil {
				cn.err.set(driver.ErrBadConn)
				return nil, fmt.Errorf("pq: unexpected message %q in simple query execution", t)
			}
			if res == nil {
				res = &rows{cn: cn}
			}
			// Set the result and tag to the last command complete if there wasn't a
			// query already run. Although queries usually return from here and cede
			// control to Next, a query with zero results does not.
			if t == proto.CommandComplete {
				res.result, res.tag, err = cn.parseComplete(r.string())
				if err != nil {
					return nil, cn.handleError(err, q)
				}
				if res.colNames != nil {
					return res, cn.handleError(resErr, q)
				}
			}
			res.done = true
		case proto.ReadyForQuery:
			cn.processReadyForQuery(r)
			if err == nil && res == nil {
				res = &rows{done: true}
			}
			return res, cn.handleError(resErr, q) // done
		case proto.ErrorResponse:
			res = nil
			resErr = parseError(r, q)
		case proto.DataRow:
			if res == nil {
				cn.err.set(driver.ErrBadConn)
				return nil, fmt.Errorf("pq: unexpected DataRow in simple query execution")
			}
			return res, cn.saveMessage(t, r) // The query didn't fail; kick off to Next
		case proto.RowDescription:
			// res might be non-nil here if we received a previous
			// CommandComplete, but that's fine and just overwrite it.
			res = &rows{cn: cn, rowsHeader: parsePortalRowDescribe(r)}

			// To work around a bug in QueryRow in Go 1.2 and earlier, wait
			// until the first DataRow has been received.
		default:
			cn.err.set(driver.ErrBadConn)
			return nil, fmt.Errorf("pq: unknown response for simple query: %q", t)
		}
	}
}

// Decides which column formats to use for a prepared statement.  The input is
// an array of type oids, one element per result column.
func decideColumnFormats(colTyps []fieldDesc, forceText bool) (colFmts []format, colFmtData []byte, _ error) {
	if len(colTyps) == 0 {
		return nil, colFmtDataAllText, nil
	}

	colFmts = make([]format, len(colTyps))
	if forceText {
		return colFmts, colFmtDataAllText, nil
	}

	allBinary := true
	allText := true
	for i, t := range colTyps {
		switch t.OID {
		// This is the list of types to use binary mode for when receiving them
		// through a prepared statement.  If a type appears in this list, it
		// must also be implemented in binaryDecode in encode.go.
		case oid.T_bytea:
			fallthrough
		case oid.T_int8:
			fallthrough
		case oid.T_int4:
			fallthrough
		case oid.T_int2:
			fallthrough
		case oid.T_uuid:
			colFmts[i] = formatBinary
			allText = false
		default:
			allBinary = false
		}
	}

	if allBinary {
		return colFmts, colFmtDataAllBinary, nil
	} else if allText {
		return colFmts, colFmtDataAllText, nil
	} else {
		colFmtData = make([]byte, 2+len(colFmts)*2)
		if len(colFmts) > math.MaxUint16 {
			return nil, nil, fmt.Errorf("pq: too many columns (%d > math.MaxUint16)", len(colFmts))
		}
		binary.BigEndian.PutUint16(colFmtData, uint16(len(colFmts)))
		for i, v := range colFmts {
			binary.BigEndian.PutUint16(colFmtData[2+i*2:], uint16(v))
		}
		return colFmts, colFmtData, nil
	}
}

func (cn *conn) prepareTo(q, stmtName string) (*stmt, error) {
	if debugProto {
		fmt.Fprintln(os.Stderr, "         START conn.prepareTo")
		defer fmt.Fprintln(os.Stderr, "         END conn.prepareTo")
	}

	st := &stmt{cn: cn, name: stmtName}

	b := cn.writeBuf(proto.Parse)
	b.string(st.name)
	b.string(q)
	b.int16(0)

	b.next(proto.Describe)
	b.byte(proto.Sync)
	b.string(st.name)

	b.next(proto.Sync)
	err := cn.send(b)
	if err != nil {
		return nil, err
	}

	err = cn.readParseResponse()
	if err != nil {
		return nil, err
	}
	st.paramTyps, st.colNames, st.colTyps, err = cn.readStatementDescribeResponse()
	if err != nil {
		return nil, err
	}
	st.colFmts, st.colFmtData, err = decideColumnFormats(st.colTyps, cn.cfg.DisablePreparedBinaryResult)
	if err != nil {
		return nil, err
	}

	err = cn.readReadyForQuery()
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (cn *conn) Prepare(q string) (driver.Stmt, error) {
	if err := cn.err.get(); err != nil {
		return nil, err
	}

	if pqsql.StartsWithCopy(q) {
		s, err := cn.prepareCopyIn(q)
		if err == nil {
			cn.inProgress.Store(true)
		}
		return s, cn.handleError(err, q)
	}
	s, err := cn.prepareTo(q, cn.gname())
	if err != nil {
		return nil, cn.handleError(err, q)
	}
	return s, nil
}

func (cn *conn) Close() error {
	// Don't go through send(); ListenerConn relies on us not scribbling on the
	// scratch buffer of this connection.
	err := cn.sendSimpleMessage(proto.Terminate)
	if err != nil {
		_ = cn.c.Close() // Ensure that cn.c.Close is always run.
		return cn.handleError(err)
	}
	return cn.c.Close()
}

func toNamedValue(v []driver.Value) []driver.NamedValue {
	v2 := make([]driver.NamedValue, len(v))
	for i := range v {
		v2[i] = driver.NamedValue{Ordinal: i + 1, Value: v[i]}
	}
	return v2
}

// CheckNamedValue implements [driver.NamedValueChecker].
func (cn *conn) CheckNamedValue(nv *driver.NamedValue) error {
	if cn.cfg.BinaryParameters {
		if bin, ok := nv.Value.(interface{ BinaryValue() ([]byte, error) }); ok {
			var err error
			nv.Value, err = bin.BinaryValue()
			return err
		}
	}

	// Ignore Valuer, for backward compatibility with pq.Array().
	if _, ok := nv.Value.(driver.Valuer); ok {
		return driver.ErrSkip
	}

	v := reflect.ValueOf(nv.Value)
	if !v.IsValid() {
		return driver.ErrSkip
	}
	t := v.Type()
	for t.Kind() == reflect.Pointer {
		t, v = t.Elem(), v.Elem()
	}

	// Ignore []byte and related types: *[]byte, json.RawMessage, etc.
	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		return driver.ErrSkip
	}

	switch v.Kind() {
	default:
		return driver.ErrSkip
	case reflect.Slice:
		var err error
		nv.Value, err = Array(v.Interface()).Value()
		return err
	case reflect.Uint64:
		value := v.Uint()
		if value >= math.MaxInt64 {
			nv.Value = strconv.FormatUint(value, 10)
		} else {
			nv.Value = int64(value)
		}
		return nil
	}
}

// Implement the "Queryer" interface
func (cn *conn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return cn.query(query, toNamedValue(args))
}

func (cn *conn) query(query string, args []driver.NamedValue) (*rows, error) {
	if debugProto {
		fmt.Fprintln(os.Stderr, "         START conn.query")
		defer fmt.Fprintln(os.Stderr, "         END conn.query")
	}
	if err := cn.err.get(); err != nil {
		return nil, err
	}
	if !cn.inProgress.CompareAndSwap(false, true) {
		return nil, errQueryInProgress
	}

	// Check to see if we can use the "simpleQuery" interface, which is
	// *much* faster than going through prepare/exec
	if len(args) == 0 {
		return cn.simpleQuery(query)
	}

	if cn.cfg.BinaryParameters {
		err := cn.sendBinaryModeQuery(query, args)
		if err != nil {
			return nil, cn.handleError(err, query)
		}
		err = cn.readParseResponse()
		if err != nil {
			return nil, cn.handleError(err, query)
		}
		err = cn.readBindResponse()
		if err != nil {
			return nil, cn.handleError(err, query)
		}

		rows := &rows{cn: cn}
		rows.rowsHeader, err = cn.readPortalDescribeResponse()
		if err != nil {
			return nil, cn.handleError(err, query)
		}
		err = cn.postExecuteWorkaround()
		if err != nil {
			return nil, cn.handleError(err, query)
		}
		return rows, nil
	}

	st, err := cn.prepareTo(query, "")
	if err != nil {
		return nil, cn.handleError(err, query)
	}
	err = st.exec(args)
	if err != nil {
		return nil, cn.handleError(err, query)
	}
	return &rows{
		cn:         cn,
		rowsHeader: st.rowsHeader,
	}, nil
}

// Implement the optional "Execer" interface for one-shot queries
func (cn *conn) Exec(query string, args []driver.Value) (driver.Result, error) {
	if err := cn.err.get(); err != nil {
		return nil, err
	}
	if !cn.inProgress.CompareAndSwap(false, true) {
		return nil, errQueryInProgress
	}

	// Check to see if we can use the "simpleExec" interface, which is *much*
	// faster than going through prepare/exec
	if len(args) == 0 {
		// ignore commandTag, our caller doesn't care
		r, _, err := cn.simpleExec(query)
		return r, cn.handleError(err, query)
	}

	if cn.cfg.BinaryParameters {
		err := cn.sendBinaryModeQuery(query, toNamedValue(args))
		if err != nil {
			return nil, cn.handleError(err, query)
		}
		err = cn.readParseResponse()
		if err != nil {
			return nil, cn.handleError(err, query)
		}
		err = cn.readBindResponse()
		if err != nil {
			return nil, cn.handleError(err, query)
		}

		_, err = cn.readPortalDescribeResponse()
		if err != nil {
			return nil, cn.handleError(err, query)
		}
		err = cn.postExecuteWorkaround()
		if err != nil {
			return nil, cn.handleError(err, query)
		}
		res, _, err := cn.readExecuteResponse("Execute")
		return res, cn.handleError(err, query)
	}

	// Use the unnamed statement to defer planning until bind time, or else
	// value-based selectivity estimates cannot be used.
	st, err := cn.prepareTo(query, "")
	if err != nil {
		return nil, cn.handleError(err, query)
	}
	r, err := st.Exec(args)
	if err != nil {
		return nil, cn.handleError(err, query)
	}
	return r, nil
}

type safeRetryError struct{ Err error }

func (se *safeRetryError) Error() string { return se.Err.Error() }

func (cn *conn) send(m *writeBuf) error {
	if debugProto {
		w := m.wrap()
		for len(w) > 0 { // Can contain multiple messages.
			c := proto.RequestCode(w[0])
			l := int(binary.BigEndian.Uint32(w[1:5])) - 4
			fmt.Fprintf(os.Stderr, "CLIENT → %-20s %5d  %q\n", c, l, w[5:l+5])
			w = w[l+5:]
		}
	}

	n, err := cn.c.Write(m.wrap())
	if err != nil && n == 0 {
		err = &safeRetryError{Err: err}
	}
	return err
}

func (cn *conn) sendStartupPacket(m *writeBuf) error {
	if debugProto {
		w := m.wrap()
		fmt.Fprintf(os.Stderr, "CLIENT → %-20s %5d  %q\n", "Startup", int(binary.BigEndian.Uint32(w[1:5]))-4, w[5:])
	}
	_, err := cn.c.Write((m.wrap())[1:])
	return err
}

// Send a message of type typ to the server on the other end of cn. The message
// should have no payload. This method does not use the scratch buffer.
func (cn *conn) sendSimpleMessage(typ proto.RequestCode) error {
	if debugProto {
		fmt.Fprintf(os.Stderr, "CLIENT → %-20s %5d  %q\n", typ, 0, []byte{})
	}
	_, err := cn.c.Write([]byte{byte(typ), '\x00', '\x00', '\x00', '\x04'})
	return err
}

// saveMessage memorizes a message and its buffer in the conn struct.
// recvMessage will then return these values on the next call to it.  This
// method is useful in cases where you have to see what the next message is
// going to be (e.g. to see whether it's an error or not) but you can't handle
// the message yourself.
func (cn *conn) saveMessage(typ proto.ResponseCode, buf *readBuf) error {
	if cn.saveMessageType != 0 {
		cn.err.set(driver.ErrBadConn)
		return fmt.Errorf("unexpected saveMessageType %d", cn.saveMessageType)
	}
	cn.saveMessageType = typ
	cn.saveMessageBuffer = *buf
	return nil
}

// recvMessage receives any message from the backend, or returns an error if
// a problem occurred while reading the message.
func (cn *conn) recvMessage(r *readBuf) (proto.ResponseCode, error) {
	// workaround for a QueryRow bug, see exec
	if cn.saveMessageType != 0 {
		t := cn.saveMessageType
		*r = cn.saveMessageBuffer
		cn.saveMessageType = 0
		cn.saveMessageBuffer = nil
		return t, nil
	}

	x := cn.scratch[:5]
	_, err := io.ReadFull(cn.buf, x)
	if err != nil {
		return 0, err
	}

	// Read the type and length of the message that follows.
	t := proto.ResponseCode(x[0])
	n := int(binary.BigEndian.Uint32(x[1:])) - 4

	if proto.ResponseCode(t) == proto.ReadyForQuery {
		cn.inProgress.Store(false)
	}

	// When PostgreSQL cannot start a backend (e.g., an external process limit),
	// it sends plain text like "Ecould not fork new process [..]", which
	// doesn't use the standard encoding for the Error message.
	//
	// libpq checks "if ErrorResponse && (msgLength < 8 || msgLength > MAX_ERRLEN)",
	// but check < 4 since n represents bytes remaining to be read after length.
	if t == proto.ErrorResponse && (n < 4 || n > proto.MaxErrlen) {
		msg, _ := cn.buf.ReadString('\x00')
		return 0, fmt.Errorf("pq: server error: %s%s", string(x[1:]), strings.TrimSuffix(msg, "\x00"))
	}

	var y []byte
	if n <= len(cn.scratch) {
		y = cn.scratch[:n]
	} else {
		y = make([]byte, n)
	}
	_, err = io.ReadFull(cn.buf, y)
	if err != nil {
		return 0, err
	}
	*r = y
	if debugProto {
		fmt.Fprintf(os.Stderr, "SERVER ← %-20s %5d  %q\n", t, n, y)
	}
	return t, nil
}

// recv receives a message from the backend, returning an error if an error
// happened while reading the message or the received message an ErrorResponse.
// NoticeResponses are ignored. This function should generally be used only
// during the startup sequence.
func (cn *conn) recv() (proto.ResponseCode, *readBuf, error) {
	for {
		r := new(readBuf)
		t, err := cn.recvMessage(r)
		if err != nil {
			return 0, nil, err
		}
		switch t {
		case proto.ErrorResponse:
			return 0, nil, parseError(r, "")
		case proto.NoticeResponse:
			if n := cn.noticeHandler; n != nil {
				n(parseError(r, ""))
			}
		case proto.NotificationResponse:
			if n := cn.notificationHandler; n != nil {
				n(recvNotification(r))
			}
		default:
			return t, r, nil
		}
	}
}

// recv1Buf is exactly equivalent to recv1, except it uses a buffer supplied by
// the caller to avoid an allocation.
func (cn *conn) recv1Buf(r *readBuf) (proto.ResponseCode, error) {
	for {
		t, err := cn.recvMessage(r)
		if err != nil {
			return 0, err
		}

		switch t {
		case proto.NotificationResponse:
			if n := cn.notificationHandler; n != nil {
				n(recvNotification(r))
			}
		case proto.NoticeResponse:
			if n := cn.noticeHandler; n != nil {
				n(parseError(r, ""))
			}
		case proto.ParameterStatus:
			cn.processParameterStatus(r)
		default:
			return t, nil
		}
	}
}

// recv1 receives a message from the backend, returning an error if an error
// happened while reading the message or the received message an ErrorResponse.
// All asynchronous messages are ignored, with the exception of ErrorResponse.
func (cn *conn) recv1() (proto.ResponseCode, *readBuf, error) {
	r := new(readBuf)
	t, err := cn.recv1Buf(r)
	if err != nil {
		return 0, nil, err
	}
	return t, r, nil
}

// Don't refer to Config.SSLMode here, as the mode in arguments may be different
// in case of sslmode=allow or prefer.
func (cn *conn) ssl(cfg Config, mode SSLMode) error {
	upgrade, err := ssl(cfg, mode)
	if err != nil {
		return err
	}
	if upgrade == nil {
		return nil // Nothing to do
	}

	// Only negotiate the ssl handshake if requested (which is the default).
	// sslnegotiation=direct is supported by pg17 and above.
	if cfg.SSLNegotiation != SSLNegotiationDirect {
		w := cn.writeBuf(0)
		w.int32(proto.NegotiateSSLCode)
		if err = cn.sendStartupPacket(w); err != nil {
			return err
		}

		b := cn.scratch[:1]
		_, err = io.ReadFull(cn.c, b)
		if err != nil {
			return err
		}

		if b[0] != 'S' {
			return ErrSSLNotSupported
		}
	}

	cn.c, err = upgrade(cn.c)
	return err
}

func (cn *conn) startup(cfg Config) error {
	w := cn.writeBuf(0)
	// Send maximum protocol version in startup; if the server doesn't support
	// this version it responds with NegotiateProtocolVersion and the maximum
	// version it supports (and will use).
	w.int32(cfg.MaxProtocolVersion.proto())

	if cfg.User != "" {
		w.string("user")
		w.string(cfg.User)
	}
	if cfg.Database != "" {
		w.string("database")
		w.string(cfg.Database)
	}
	// w.string("replication") // Sent by libpq, but we don't support that.
	if cfg.Options != "" {
		w.string("options")
		w.string(cfg.Options)
	}
	if cfg.ApplicationName != "" {
		w.string("application_name")
		w.string(cfg.ApplicationName)
	}
	if cfg.ClientEncoding != "" {
		w.string("client_encoding")
		w.string(cfg.ClientEncoding)
	}
	if cfg.Datestyle != "" {
		w.string("datestyle")
		w.string(cfg.Datestyle)
	}
	for k, v := range cfg.Runtime {
		w.string(k)
		w.string(v)
	}

	w.string("")
	if err := cn.sendStartupPacket(w); err != nil {
		return err
	}

	for {
		t, r, err := cn.recv()
		if err != nil {
			return err
		}
		switch t {
		case proto.BackendKeyData:
			cn.pid = r.int32()
			if len(*r) > 256 {
				return fmt.Errorf("pq: cancellation key longer than 256 bytes: %d bytes", len(*r))
			}
			cn.secretKey = make([]byte, len(*r))
			copy(cn.secretKey, *r)
		case proto.ParameterStatus:
			cn.processParameterStatus(r)
		case proto.AuthenticationRequest:
			err := cn.auth(r, cfg)
			if err != nil {
				return err
			}
		case proto.NegotiateProtocolVersion:
			newestMinor := r.int32()
			serverVersion := proto.ProtocolVersion30&0xFFFF0000 | newestMinor
			if serverVersion < cfg.MinProtocolVersion.proto() {
				return fmt.Errorf("pq: protocol version mismatch: min_protocol_version=%s; server supports up to 3.%d", cfg.MinProtocolVersion, newestMinor)
			}
		case proto.ReadyForQuery:
			cn.processReadyForQuery(r)
			return nil
		default:
			return fmt.Errorf("pq: unknown response for startup: %q", t)
		}
	}
}

func (cn *conn) auth(r *readBuf, cfg Config) error {
	switch code := proto.AuthCode(r.int32()); code {
	default:
		return fmt.Errorf("pq: unknown authentication response: %s", code)
	case proto.AuthReqKrb4, proto.AuthReqKrb5, proto.AuthReqCrypt, proto.AuthReqSSPI:
		return fmt.Errorf("pq: unsupported authentication method: %s", code)
	case proto.AuthReqOk:
		return nil

	case proto.AuthReqPassword:
		w := cn.writeBuf(proto.PasswordMessage)
		w.string(cfg.Password)
		// Don't need to check AuthOk response here; auth() is called in a loop,
		// which catches the errors and AuthReqOk responses.
		return cn.send(w)

	case proto.AuthReqMD5:
		s := string(r.next(4))
		w := cn.writeBuf(proto.PasswordMessage)
		w.string("md5" + md5s(md5s(cfg.Password+cfg.User)+s))
		// Same here.
		return cn.send(w)

	case proto.AuthReqGSS: // GSSAPI, startup
		if newGss == nil {
			return fmt.Errorf("pq: kerberos error: no GSSAPI provider registered (import github.com/lib/pq/auth/kerberos)")
		}
		cli, err := newGss()
		if err != nil {
			return fmt.Errorf("pq: kerberos error: %w", err)
		}

		var token []byte
		if cfg.KrbSpn != "" {
			// Use the supplied SPN if provided.
			token, err = cli.GetInitTokenFromSpn(cfg.KrbSpn)
		} else {
			// Allow the kerberos service name to be overridden.
			service := "postgres"
			if cfg.KrbSrvname != "" {
				service = cfg.KrbSrvname
			}
			token, err = cli.GetInitToken(cfg.Host, service)
		}
		if err != nil {
			return fmt.Errorf("pq: failed to get Kerberos ticket: %w", err)
		}

		w := cn.writeBuf(proto.GSSResponse)
		w.bytes(token)
		err = cn.send(w)
		if err != nil {
			return err
		}

		// Store for GSSAPI continue message
		cn.gss = cli
		return nil

	case proto.AuthReqGSSCont: // GSSAPI continue
		if cn.gss == nil {
			return errors.New("pq: GSSAPI protocol error")
		}

		done, tokOut, err := cn.gss.Continue([]byte(*r))
		if err == nil && !done {
			w := cn.writeBuf(proto.SASLInitialResponse)
			w.bytes(tokOut)
			err = cn.send(w)
			if err != nil {
				return err
			}
		}

		// Errors fall through and read the more detailed message from the
		// server.
		return nil

	case proto.AuthReqSASL:
		sc := scram.NewClient(sha256.New, cfg.User, cfg.Password)
		sc.Step(nil)
		if sc.Err() != nil {
			return fmt.Errorf("pq: SCRAM-SHA-256 error: %w", sc.Err())
		}
		scOut := sc.Out()

		w := cn.writeBuf(proto.SASLResponse)
		w.string("SCRAM-SHA-256")
		w.int32(len(scOut))
		w.bytes(scOut)
		err := cn.send(w)
		if err != nil {
			return err
		}

		t, r, err := cn.recv()
		if err != nil {
			return err
		}
		if t != proto.AuthenticationRequest {
			return fmt.Errorf("pq: unexpected password response: %q", t)
		}

		if r.int32() != int(proto.AuthReqSASLCont) {
			return fmt.Errorf("pq: unexpected authentication response: %q", t)
		}

		nextStep := r.next(len(*r))
		sc.Step(nextStep)
		if sc.Err() != nil {
			return fmt.Errorf("pq: SCRAM-SHA-256 error: %w", sc.Err())
		}

		scOut = sc.Out()
		w = cn.writeBuf(proto.SASLResponse)
		w.bytes(scOut)
		err = cn.send(w)
		if err != nil {
			return err
		}

		t, r, err = cn.recv()
		if err != nil {
			return err
		}
		if t != proto.AuthenticationRequest {
			return fmt.Errorf("pq: unexpected password response: %q", t)
		}

		if r.int32() != int(proto.AuthReqSASLFin) {
			return fmt.Errorf("pq: unexpected authentication response: %q", t)
		}

		nextStep = r.next(len(*r))
		sc.Step(nextStep)
		if sc.Err() != nil {
			return fmt.Errorf("pq: SCRAM-SHA-256 error: %w", sc.Err())
		}

		return nil
	}
}

// parseComplete parses the "command tag" from a CommandComplete message, and
// returns the number of rows affected (if applicable) and a string identifying
// only the command that was executed, e.g. "ALTER TABLE". Returns an error if
// the command can cannot be parsed.
func (cn *conn) parseComplete(commandTag string) (driver.Result, string, error) {
	commandsWithAffectedRows := []string{
		"SELECT ",
		// INSERT is handled below
		"UPDATE ",
		"DELETE ",
		"FETCH ",
		"MOVE ",
		"COPY ",
	}

	var affectedRows *string
	for _, tag := range commandsWithAffectedRows {
		if strings.HasPrefix(commandTag, tag) {
			t := commandTag[len(tag):]
			affectedRows = &t
			commandTag = tag[:len(tag)-1]
			break
		}
	}
	// INSERT also includes the oid of the inserted row in its command tag. Oids
	// in user tables are deprecated, and the oid is only returned when exactly
	// one row is inserted, so it's unlikely to be of value to any real-world
	// application and we can ignore it.
	if affectedRows == nil && strings.HasPrefix(commandTag, "INSERT ") {
		parts := strings.Split(commandTag, " ")
		if len(parts) != 3 {
			cn.err.set(driver.ErrBadConn)
			return nil, "", fmt.Errorf("pq: unexpected INSERT command tag %s", commandTag)
		}
		affectedRows = &parts[len(parts)-1]
		commandTag = "INSERT"
	}
	// There should be no affected rows attached to the tag, just return it
	if affectedRows == nil {
		return driver.RowsAffected(0), commandTag, nil
	}
	n, err := strconv.ParseInt(*affectedRows, 10, 64)
	if err != nil {
		cn.err.set(driver.ErrBadConn)
		return nil, "", fmt.Errorf("pq: could not parse commandTag: %w", err)
	}
	return driver.RowsAffected(n), commandTag, nil
}

func md5s(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (cn *conn) sendBinaryParameters(b *writeBuf, args []driver.NamedValue) error {
	// Do one pass over the parameters to see if we're going to send any of them
	// over in binary. If we are, create a paramFormats array at the same time.
	var paramFormats []int
	for i, x := range args {
		_, ok := x.Value.([]byte)
		if ok {
			if paramFormats == nil {
				paramFormats = make([]int, len(args))
			}
			paramFormats[i] = 1
		}
	}
	if paramFormats == nil {
		b.int16(0)
	} else {
		b.int16(len(paramFormats))
		for _, x := range paramFormats {
			b.int16(x)
		}
	}

	b.int16(len(args))
	for _, x := range args {
		if x.Value == nil {
			b.int32(-1)
		} else if xx, ok := x.Value.([]byte); ok && xx == nil {
			b.int32(-1)
		} else {
			datum, err := binaryEncode(x.Value)
			if err != nil {
				return err
			}
			b.int32(len(datum))
			b.bytes(datum)
		}
	}
	return nil
}

func (cn *conn) sendBinaryModeQuery(query string, args []driver.NamedValue) error {
	if len(args) >= 65536 {
		return fmt.Errorf("pq: got %d parameters but PostgreSQL only supports 65535 parameters", len(args))
	}

	b := cn.writeBuf(proto.Parse)
	b.byte(0) // unnamed statement
	b.string(query)
	b.int16(0)

	b.next(proto.Bind)
	b.int16(0) // unnamed portal and statement
	err := cn.sendBinaryParameters(b, args)
	if err != nil {
		return err
	}
	b.bytes(colFmtDataAllText)

	b.next(proto.Describe)
	b.byte(proto.Parse)
	b.byte(0) // unnamed portal

	b.next(proto.Execute)
	b.byte(0)
	b.int32(0)

	b.next(proto.Sync)
	return cn.send(b)
}

func (cn *conn) processParameterStatus(r *readBuf) {
	switch r.string() {
	default:
		// ignore
	case "server_version":
		var major1, major2 int
		_, err := fmt.Sscanf(r.string(), "%d.%d", &major1, &major2)
		if err == nil {
			cn.parameterStatus.serverVersion = major1*10000 + major2*100
		}
	case "TimeZone":
		switch tz := r.string(); tz {
		case "UTC", "Etc/UTC", "Etc/Universal", "Etc/Zulu", "Etc/UCT":
			cn.parameterStatus.currentLocation = time.UTC
		default:
			var err error
			cn.parameterStatus.currentLocation, err = time.LoadLocation(tz)
			if err != nil {
				cn.parameterStatus.currentLocation = nil
			}
		}
	// Use sql.NullBool so we can distinguish between false and not sent. If
	// it's not sent we use a query to get the value – I don't know when these
	// parameters are not sent, but this is what libpq does.
	case "in_hot_standby":
		b, err := pqutil.ParseBool(r.string())
		if err == nil {
			cn.parameterStatus.inHotStandby = sql.NullBool{Valid: true, Bool: b}
		}
	case "default_transaction_read_only":
		b, err := pqutil.ParseBool(r.string())
		if err == nil {
			cn.parameterStatus.defaultTransactionReadOnly = sql.NullBool{Valid: true, Bool: b}
		}
	}
}

func (cn *conn) processReadyForQuery(r *readBuf) {
	cn.txnStatus = transactionStatus(r.byte())
}

func (cn *conn) readReadyForQuery() error {
	t, r, err := cn.recv1()
	if err != nil {
		return err
	}
	switch t {
	case proto.ReadyForQuery:
		cn.processReadyForQuery(r)
		return nil
	case proto.ErrorResponse:
		err := parseError(r, "")
		cn.err.set(driver.ErrBadConn)
		return err
	default:
		cn.err.set(driver.ErrBadConn)
		return fmt.Errorf("pq: unexpected message %q; expected ReadyForQuery", t)
	}
}

func (cn *conn) readParseResponse() error {
	t, r, err := cn.recv1()
	if err != nil {
		return err
	}
	switch t {
	case proto.ParseComplete:
		return nil
	case proto.ErrorResponse:
		err := parseError(r, "")
		_ = cn.readReadyForQuery()
		return err
	default:
		cn.err.set(driver.ErrBadConn)
		return fmt.Errorf("pq: unexpected Parse response %q", t)
	}
}

func (cn *conn) readStatementDescribeResponse() (paramTyps []oid.Oid, colNames []string, colTyps []fieldDesc, _ error) {
	for {
		t, r, err := cn.recv1()
		if err != nil {
			return nil, nil, nil, err
		}
		switch t {
		case proto.ParameterDescription:
			nparams := r.int16()
			paramTyps = make([]oid.Oid, nparams)
			for i := range paramTyps {
				paramTyps[i] = r.oid()
			}
		case proto.NoData:
			return paramTyps, nil, nil, nil
		case proto.RowDescription:
			colNames, colTyps = parseStatementRowDescribe(r)
			return paramTyps, colNames, colTyps, nil
		case proto.ErrorResponse:
			err := parseError(r, "")
			_ = cn.readReadyForQuery()
			return nil, nil, nil, err
		default:
			cn.err.set(driver.ErrBadConn)
			return nil, nil, nil, fmt.Errorf("pq: unexpected Describe statement response %q", t)
		}
	}
}

func (cn *conn) readPortalDescribeResponse() (rowsHeader, error) {
	t, r, err := cn.recv1()
	if err != nil {
		return rowsHeader{}, err
	}
	switch t {
	case proto.RowDescription:
		return parsePortalRowDescribe(r), nil
	case proto.NoData:
		return rowsHeader{}, nil
	case proto.ErrorResponse:
		err := parseError(r, "")
		_ = cn.readReadyForQuery()
		return rowsHeader{}, err
	default:
		cn.err.set(driver.ErrBadConn)
		return rowsHeader{}, fmt.Errorf("pq: unexpected Describe response %q", t)
	}
}

func (cn *conn) readBindResponse() error {
	t, r, err := cn.recv1()
	if err != nil {
		return err
	}
	switch t {
	case proto.BindComplete:
		return nil
	case proto.ErrorResponse:
		err := parseError(r, "")
		_ = cn.readReadyForQuery()
		return err
	default:
		cn.err.set(driver.ErrBadConn)
		return fmt.Errorf("pq: unexpected Bind response %q", t)
	}
}

func (cn *conn) postExecuteWorkaround() error {
	// Work around a bug in sql.DB.QueryRow: in Go 1.2 and earlier it ignores
	// any errors from rows.Next, which masks errors that happened during the
	// execution of the query.  To avoid the problem in common cases, we wait
	// here for one more message from the database.  If it's not an error the
	// query will likely succeed (or perhaps has already, if it's a
	// CommandComplete), so we push the message into the conn struct; recv1
	// will return it as the next message for rows.Next or rows.Close.
	// However, if it's an error, we wait until ReadyForQuery and then return
	// the error to our caller.
	for {
		t, r, err := cn.recv1()
		if err != nil {
			return err
		}
		switch t {
		case proto.ErrorResponse:
			err := parseError(r, "")
			_ = cn.readReadyForQuery()
			return err
		case proto.CommandComplete, proto.DataRow, proto.EmptyQueryResponse:
			// the query didn't fail, but we can't process this message
			return cn.saveMessage(t, r)
		default:
			cn.err.set(driver.ErrBadConn)
			return fmt.Errorf("pq: unexpected message during extended query execution: %q", t)
		}
	}
}

// Only for Exec(), since we ignore the returned data
func (cn *conn) readExecuteResponse(protocolState string) (res driver.Result, commandTag string, resErr error) {
	for {
		t, r, err := cn.recv1()
		if err != nil {
			return nil, "", err
		}
		switch t {
		case proto.CommandComplete:
			if resErr != nil {
				cn.err.set(driver.ErrBadConn)
				return nil, "", fmt.Errorf("pq: unexpected CommandComplete after error %s", resErr)
			}
			res, commandTag, err = cn.parseComplete(r.string())
			if err != nil {
				return nil, "", err
			}
		case proto.ReadyForQuery:
			cn.processReadyForQuery(r)
			if res == nil && resErr == nil {
				resErr = errUnexpectedReady
			}
			return res, commandTag, resErr
		case proto.ErrorResponse:
			resErr = parseError(r, "")
		case proto.RowDescription, proto.DataRow, proto.EmptyQueryResponse:
			if resErr != nil {
				cn.err.set(driver.ErrBadConn)
				return nil, "", fmt.Errorf("pq: unexpected %q after error %s", t, resErr)
			}
			if t == proto.EmptyQueryResponse {
				res = emptyRows
			}
			// ignore any results
		default:
			cn.err.set(driver.ErrBadConn)
			return nil, "", fmt.Errorf("pq: unknown %s response: %q", protocolState, t)
		}
	}
}

func parseStatementRowDescribe(r *readBuf) (colNames []string, colTyps []fieldDesc) {
	n := r.int16()
	colNames = make([]string, n)
	colTyps = make([]fieldDesc, n)
	for i := range colNames {
		colNames[i] = r.string()
		r.next(6)
		colTyps[i].OID = r.oid()
		colTyps[i].Len = r.int16()
		colTyps[i].Mod = r.int32()
		// format code not known when describing a statement; always 0
		r.next(2)
	}
	return
}

func parsePortalRowDescribe(r *readBuf) rowsHeader {
	n := r.int16()
	colNames := make([]string, n)
	colFmts := make([]format, n)
	colTyps := make([]fieldDesc, n)
	for i := range colNames {
		colNames[i] = r.string()
		r.next(6)
		colTyps[i].OID = r.oid()
		colTyps[i].Len = r.int16()
		colTyps[i].Mod = r.int32()
		colFmts[i] = format(r.int16())
	}
	return rowsHeader{
		colNames: colNames,
		colFmts:  colFmts,
		colTyps:  colTyps,
	}
}

func (cn *conn) ResetSession(ctx context.Context) error {
	// Ensure bad connections are reported: From database/sql/driver:
	// If a connection is never returned to the connection pool but immediately reused, then
	// ResetSession is called prior to reuse but IsValid is not called.
	return cn.err.get()
}

func (cn *conn) IsValid() bool {
	return cn.err.get() == nil
}

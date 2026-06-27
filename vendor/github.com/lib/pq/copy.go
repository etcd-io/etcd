package pq

import (
	"context"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/lib/pq/internal/proto"
)

var (
	errCopyInClosed               = errors.New("pq: copyin statement has already been closed")
	errBinaryCopyNotSupported     = errors.New("pq: only text format supported for COPY")
	errCopyToNotSupported         = errors.New("pq: COPY TO is not supported")
	errCopyNotSupportedOutsideTxn = errors.New("pq: COPY is only allowed inside a transaction")
)

type copyin struct {
	cn      *conn
	buffer  []byte
	rowData chan []byte
	done    chan bool
	closed  bool
	mu      struct {
		sync.Mutex
		err error
		driver.Result
	}
}

const (
	ciBufferSize = 64 * 1024
	// flush buffer before the buffer is filled up and needs reallocation
	ciBufferFlushSize = 63 * 1024
)

func (cn *conn) prepareCopyIn(q string) (_ driver.Stmt, resErr error) {
	if !cn.isInTransaction() {
		return nil, errCopyNotSupportedOutsideTxn
	}

	ci := &copyin{
		cn:      cn,
		buffer:  make([]byte, 0, ciBufferSize),
		rowData: make(chan []byte),
		done:    make(chan bool, 1),
	}
	// add CopyData identifier + 4 bytes for message length
	ci.buffer = append(ci.buffer, byte(proto.CopyDataRequest), 0, 0, 0, 0)

	b := cn.writeBuf(proto.Query)
	b.string(q)
	err := cn.send(b)
	if err != nil {
		return nil, err
	}

awaitCopyInResponse:
	for {
		t, r, err := cn.recv1()
		if err != nil {
			return nil, err
		}
		switch t {
		case proto.CopyInResponse:
			if r.byte() != 0 {
				resErr = errBinaryCopyNotSupported
				break awaitCopyInResponse
			}
			go ci.resploop()
			return ci, nil
		case proto.CopyOutResponse:
			resErr = errCopyToNotSupported
			break awaitCopyInResponse
		case proto.ErrorResponse:
			resErr = parseError(r, q)
		case proto.ReadyForQuery:
			if resErr == nil {
				ci.setBad(driver.ErrBadConn)
				return nil, fmt.Errorf("pq: unexpected ReadyForQuery in response to COPY")
			}
			cn.processReadyForQuery(r)
			return nil, resErr
		default:
			ci.setBad(driver.ErrBadConn)
			return nil, fmt.Errorf("pq: unknown response for copy query: %q", t)
		}
	}

	// something went wrong, abort COPY before we return
	b = cn.writeBuf(proto.CopyFail)
	b.string(resErr.Error())
	err = cn.send(b)
	if err != nil {
		return nil, err
	}

	for {
		t, r, err := cn.recv1()
		if err != nil {
			return nil, err
		}

		switch t {
		case proto.CopyDoneResponse, proto.CommandComplete, proto.ErrorResponse:
		case proto.ReadyForQuery:
			// correctly aborted, we're done
			cn.processReadyForQuery(r)
			return nil, resErr
		default:
			ci.setBad(driver.ErrBadConn)
			return nil, fmt.Errorf("pq: unknown response for CopyFail: %q", t)
		}
	}
}

func (ci *copyin) flush(buf []byte) error {
	if len(buf)-1 > proto.MaxUint32 {
		return errors.New("pq: too many columns")
	}
	if debugProto {
		fmt.Fprintf(os.Stderr, "CLIENT → %-20s %5d  %q\n", proto.RequestCode(buf[0]), len(buf)-5, buf[5:])
	}
	binary.BigEndian.PutUint32(buf[1:], uint32(len(buf)-1)) // Set message length (without message identifier).
	_, err := ci.cn.c.Write(buf)
	return err
}

func (ci *copyin) resploop() {
	for {
		var r readBuf
		t, err := ci.cn.recvMessage(&r)
		if err != nil {
			ci.setBad(driver.ErrBadConn)
			ci.setError(err)
			ci.done <- true
			return
		}
		switch t {
		case proto.CommandComplete:
			// complete
			res, _, err := ci.cn.parseComplete(r.string())
			if err != nil {
				panic(err)
			}
			ci.setResult(res)
		case proto.NoticeResponse:
			if n := ci.cn.noticeHandler; n != nil {
				n(parseError(&r, ""))
			}
		case proto.ReadyForQuery:
			ci.cn.processReadyForQuery(&r)
			ci.done <- true
			return
		case proto.ErrorResponse:
			err := parseError(&r, "")
			ci.setError(err)
		default:
			ci.setBad(driver.ErrBadConn)
			ci.setError(fmt.Errorf("unknown response during CopyIn: %q", t))
			ci.done <- true
			return
		}
	}
}

func (ci *copyin) setBad(err error) {
	ci.cn.err.set(err)
}

func (ci *copyin) getBad() error {
	return ci.cn.err.get()
}

func (ci *copyin) err() error {
	ci.mu.Lock()
	err := ci.mu.err
	ci.mu.Unlock()
	return err
}

// setError() sets ci.err if one has not been set already.  Caller must not be
// holding ci.Mutex.
func (ci *copyin) setError(err error) {
	ci.mu.Lock()
	if ci.mu.err == nil {
		ci.mu.err = err
	}
	ci.mu.Unlock()
}

func (ci *copyin) setResult(result driver.Result) {
	ci.mu.Lock()
	ci.mu.Result = result
	ci.mu.Unlock()
}

func (ci *copyin) getResult() driver.Result {
	ci.mu.Lock()
	result := ci.mu.Result
	ci.mu.Unlock()
	if result == nil {
		return driver.RowsAffected(0)
	}
	return result
}

func (ci *copyin) NumInput() int {
	return -1
}

func (ci *copyin) Query(v []driver.Value) (r driver.Rows, err error) {
	return nil, ErrNotSupported
}

// Exec inserts values into the COPY stream. The insert is asynchronous
// and Exec can return errors from previous Exec calls to the same
// COPY stmt.
//
// You need to call Exec(nil) to sync the COPY stream and to get any
// errors from pending data, since Stmt.Close() doesn't return errors
// to the user.
func (ci *copyin) Exec(v []driver.Value) (driver.Result, error) {
	if ci.closed {
		return nil, errCopyInClosed
	}
	if err := ci.getBad(); err != nil {
		return nil, err
	}
	if err := ci.err(); err != nil {
		return nil, err
	}

	if len(v) == 0 {
		if err := ci.Close(); err != nil {
			return driver.RowsAffected(0), err
		}
		return ci.getResult(), nil
	}

	var (
		numValues = len(v)
		err       error
	)
	for i, value := range v {
		ci.buffer, err = appendEncodedText(ci.buffer, value)
		if err != nil {
			return nil, ci.cn.handleError(err)
		}
		if i < numValues-1 {
			ci.buffer = append(ci.buffer, '\t')
		}
	}

	ci.buffer = append(ci.buffer, '\n')

	if len(ci.buffer) > ciBufferFlushSize {
		err := ci.flush(ci.buffer)
		if err != nil {
			return nil, ci.cn.handleError(err)
		}
		// reset buffer, keep bytes for message identifier and length
		ci.buffer = ci.buffer[:5]
	}

	return driver.RowsAffected(0), nil
}

// CopyData inserts a raw string into the COPY stream. The insert is
// asynchronous and CopyData can return errors from previous CopyData calls to
// the same COPY stmt.
//
// You need to call Exec(nil) to sync the COPY stream and to get any
// errors from pending data, since Stmt.Close() doesn't return errors
// to the user.
func (ci *copyin) CopyData(ctx context.Context, line string) (driver.Result, error) {
	if ci.closed {
		return nil, errCopyInClosed
	}
	if finish := ci.cn.watchCancel(ctx); finish != nil {
		defer finish()
	}
	if err := ci.getBad(); err != nil {
		return nil, err
	}
	if err := ci.err(); err != nil {
		return nil, err
	}

	ci.buffer = append(ci.buffer, []byte(line)...)
	ci.buffer = append(ci.buffer, '\n')

	if len(ci.buffer) > ciBufferFlushSize {
		err := ci.flush(ci.buffer)
		if err != nil {
			return nil, ci.cn.handleError(err)
		}

		// reset buffer, keep bytes for message identifier and length
		ci.buffer = ci.buffer[:5]
	}

	return driver.RowsAffected(0), nil
}

func (ci *copyin) Close() error {
	if ci.closed { // Don't do anything, we're already closed
		return nil
	}
	ci.closed = true

	if err := ci.getBad(); err != nil {
		return err
	}

	if len(ci.buffer) > 0 {
		err := ci.flush(ci.buffer)
		if err != nil {
			return ci.cn.handleError(err)
		}
	}
	// Avoid touching the scratch buffer as resploop could be using it.
	err := ci.cn.sendSimpleMessage(proto.CopyDoneRequest)
	if err != nil {
		return ci.cn.handleError(err)
	}

	<-ci.done
	ci.cn.inProgress.Store(false)

	if err := ci.err(); err != nil {
		return err
	}
	return nil
}

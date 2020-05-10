package pkg

import (
	"compress/flate"
	"database/sql/driver"
	"net/http"
	"os"
	"syscall"
)

var _ = syscall.StringByteSlice("") // want `Use ByteSliceFromString instead`

func fn1(err error) {
	var r *http.Request
	_ = r.Cancel                        // want `If a Request's Cancel field and context are both`
	_ = syscall.StringByteSlice("")     // want `Use ByteSliceFromString instead`
	_ = os.SEEK_SET                     // want `Use io\.SeekStart, io\.SeekCurrent, and io\.SeekEnd`
	if err == http.ErrWriteAfterFlush { // want `ErrWriteAfterFlush is no longer`
		println()
	}
	var _ flate.ReadError // want `No longer returned`

	var tr *http.Transport
	tr.CancelRequest(nil) // want `CancelRequest is deprecated`

	var conn driver.Conn
	conn.Begin() // want `Begin is deprecated`
}

// Deprecated: Don't use this.
func fn2() {
	_ = syscall.StringByteSlice("")

	anon := func(x int) {
		println(x)
		_ = syscall.StringByteSlice("")

		anon := func(x int) {
			println(x)
			_ = syscall.StringByteSlice("")
		}
		anon(2)
	}
	anon(1)
}

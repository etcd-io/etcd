//go:build windows && !appengine

package pqutil

import (
	"path/filepath"
	"syscall"
)

func User() (string, error) {
	// Perform Windows user name lookup identically to libpq.
	//
	// The PostgreSQL code makes use of the legacy Win32 function GetUserName,
	// and that function has not been imported into stock Go. GetUserNameEx is
	// available though, the difference being that a wider range of names are
	// available.  To get the output to be the same as GetUserName, only the
	// base (or last) component of the result is returned.
	var (
		name     = make([]uint16, 128)
		pwnameSz = uint32(len(name)) - 1
	)
	err := syscall.GetUserNameEx(syscall.NameSamCompatible, &name[0], &pwnameSz)
	if err != nil {
		return "", err
	}
	s := syscall.UTF16ToString(name)
	return filepath.Base(s), nil
}

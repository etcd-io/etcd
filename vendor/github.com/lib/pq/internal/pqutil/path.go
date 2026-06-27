package pqutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"syscall"
)

// Home gets the PostgreSQL configuration dir in the user's home directory:
// %APPDATA%/postgresql on Windows, and $HOME/.postgresql/postgresql.crt
// everywhere else.
//
// Returns an empy string if no home directory was found.
//
// Matches pqGetHomeDirectory() from PostgreSQL.
// https://github.com/postgres/postgres/blob/2b117bb/src/interfaces/libpq/fe-connect.c#L8214
func Home(subdir bool) string {
	if runtime.GOOS == "windows" {
		// pq uses SHGetFolderPath(), which is deprecated but x/sys/windows has
		// KnownFolderPath(). We don't really want to pull that in though, so
		// use APPDATA env. This is also what PostgreSQL uses in some other
		// codepaths (get_home_path() for example).
		ad := os.Getenv("APPDATA")
		if ad == "" {
			return ""
		}
		return filepath.Join(ad, "postgresql")
	}

	home, _ := os.UserHomeDir()
	if home == "" {
		u, err := user.Current()
		if err != nil {
			return ""
		}
		home = u.HomeDir
	}
	// libpq reads some files from ~/ and some from ~/.postgresql – on Windows
	// it always uses %APPDATA%/postgresql.
	if subdir {
		home = filepath.Join(home, ".postgresql")
	}
	return home
}

// ErrNotExists reports if err is a "path doesn't exist" type error.
//
// fs.ErrNotExist is not enough, as "/dev/null/somefile" will return ENOTDIR
// instead of ENOENT.
func ErrNotExists(err error) bool {
	perr := new(os.PathError)
	if errors.As(err, &perr) && (perr.Err == syscall.ENOENT || perr.Err == syscall.ENOTDIR) {
		return true
	}
	return false
}

var WarnFD io.Writer = os.Stderr

// Pgpass gets the filepath to the pgpass file to use, returning "" if a pgpass
// file shouldn't be used.
func Pgpass(passfile string) string {
	// Get passfile from the options.
	if passfile == "" {
		home := Home(false)
		if home == "" {
			return ""
		}
		passfile = filepath.Join(home, ".pgpass")
	}

	// On Win32, the directory is protected, so we don't have to check the file.
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(passfile)
		if err != nil {
			return ""
		}
		if fi.Mode().Perm()&(0x77) != 0 {
			fmt.Fprintf(WarnFD,
				"WARNING: password file %q has group or world access; permissions should be u=rw (0600) or less\n",
				passfile)
			return ""
		}
	}
	return passfile
}

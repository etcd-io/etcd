//go:build !windows && !plan9

package pqutil

import (
	"errors"
	"os"
	"syscall"
)

var (
	ErrSSLKeyUnknownOwnership    = errors.New("pq: could not get owner information for private key, may not be properly protected")
	ErrSSLKeyHasWorldPermissions = errors.New("pq: private key has world access; permissions should be u=rw,g=r (0640) if owned by root, or u=rw (0600), or less")
)

// SSLKeyPermissions checks the permissions on user-supplied SSL key files,
// which should have very little access. libpq does not check key file
// permissions on Windows.
//
// If the file is owned by the same user the process is running as, the file
// should only have 0600. If the file is owned by root, and the group matches
// the group that the process is running in, the permissions cannot be more than
// 0640. The file should never have world permissions.
//
// Returns an error when the permission check fails.
func SSLKeyPermissions(sslkey string) error {
	fi, err := os.Stat(sslkey)
	if err != nil {
		return err
	}

	return CheckPermissions(fi)
}

func CheckPermissions(fi os.FileInfo) error {
	// The maximum permissions that a private key file owned by a regular user
	// is allowed to have. This translates to u=rw. Regardless of if we're
	// running as root or not, 0600 is acceptable, so we return if no bits
	// beyond the regular user permission mask are set.
	if fi.Mode().Perm()&^os.FileMode(0o600) == 0 {
		return nil
	}

	// We need to pull the Unix file information to get the file's owner.
	// If we can't access it, there's some sort of operating system level error
	// and we should fail rather than attempting to use faulty information.
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrSSLKeyUnknownOwnership
	}

	// if the file is owned by root, we allow 0640 (u=rw,g=r) to match what
	// Postgres does.
	if sys.Uid == 0 {
		// The maximum permissions that a private key file owned by root is
		// allowed to have. This translates to u=rw,g=r.
		if fi.Mode().Perm()&^os.FileMode(0o640) != 0 {
			return ErrSSLKeyHasWorldPermissions
		}
		return nil
	}

	return ErrSSLKeyHasWorldPermissions
}

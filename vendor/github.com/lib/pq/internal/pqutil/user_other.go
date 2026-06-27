//go:build js || android || hurd || zos || wasip1 || appengine

package pqutil

import "errors"

func User() (string, error) {
	return "", errors.New("pqutil.User: not supported on current platform")
}

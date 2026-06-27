//go:build windows || plan9

package pqutil

import "errors"

var (
	ErrSSLKeyUnknownOwnership    = errors.New("unused")
	ErrSSLKeyHasWorldPermissions = errors.New("unused")
)

func SSLKeyPermissions(sslkey string) error { return nil }

//go:build go1.22
// +build go1.22

package errcheck

import "go/types"

func maybeUnalias(t types.Type) types.Type {
	return types.Unalias(t)
}

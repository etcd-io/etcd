//go:build aix
// +build aix

package filewatcher

import "golang.org/x/sys/unix"

const tcGet = unix.TCGETA
const tcSet = unix.TCSETA

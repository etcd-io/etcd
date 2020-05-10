// +build amd64 arm64 ppc64 ppc64le mips64 mips64le mips64p32 mips64p32le sparc64

package pkg

import "math"

func fn2() {
	var (
		u uint
		i int
	)
	_ = u > math.MaxUint64 // want `no value of type uint is greater than math\.MaxUint64`
	_ = i > math.MaxInt64  // want `no value of type int is greater than math\.MaxInt64`

}

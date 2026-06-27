//go:build !386 && !arm && !mips && !mipsle

package proto

import "math"

const MaxUint32 = math.MaxUint32

package stringutil

import (
	"unsafe"
)

// StringToBytes convert a string into bytes without mem-allocs.
func StringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&struct {
		string
		int
	}{s, len(s)}))
}

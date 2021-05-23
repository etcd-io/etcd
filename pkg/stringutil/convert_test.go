package stringutil

import "testing"

var str = "distributed under the License is distributed on an  AS IS BASIS,"

func BenchmarkStringToBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		StringToBytes(str)
	}
}

func BenchmarkStringToBytesRaw(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = []byte(str)
	}
}

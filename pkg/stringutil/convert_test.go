package stringutil

import (
	"bytes"
	"testing"
)

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

func TestStringToBytes(t *testing.T) {
	for i := 0; i < 1000; i++ {
		randStr := randString(uint(i))
		if !bytes.Equal(StringToBytes(randStr), []byte(randStr)) {
			t.Fatalf("string %s ,convert incorrectly", randStr)
		}
	}
}

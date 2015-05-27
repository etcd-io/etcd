package storage

import (
	"crypto/rand"
	"os"
	"testing"
)

func BenchmarkStorePut(b *testing.B) {
	s := newStore("test")
	defer os.Remove("test")

	// prepare keys
	keys := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = make([]byte, 64)
		rand.Read(keys[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put(keys[i], []byte("foo"))
	}
}

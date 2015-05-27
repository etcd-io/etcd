package backend

import (
	"crypto/rand"
	"os"
	"testing"
	"time"
)

func BenchmarkBackendPut(b *testing.B) {
	backend := New("test", 100*time.Millisecond, 10000)
	defer backend.Close()
	defer os.Remove("test")

	// prepare keys
	keys := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = make([]byte, 64)
		rand.Read(keys[i])
	}
	value := make([]byte, 128)
	rand.Read(value)

	batchTx := backend.BatchTx()

	batchTx.Lock()
	batchTx.UnsafeCreateBucket([]byte("test"))
	batchTx.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batchTx.Lock()
		batchTx.UnsafePut([]byte("test"), keys[i], value)
		batchTx.Unlock()
	}
}

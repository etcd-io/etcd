package raft

import (
	"bytes"
	"testing"
)

func BenchmarkAppendEntriesResponseEncoding(b *testing.B) {
	req, tmp := createTestAppendEntriesResponse(2000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		req.encode(&buf)
	}
	b.SetBytes(int64(len(tmp)))
}

func BenchmarkAppendEntriesResponseDecoding(b *testing.B) {
	req, buf := createTestAppendEntriesResponse(2000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.decode(bytes.NewReader(buf))
	}
	b.SetBytes(int64(len(buf)))
}

func createTestAppendEntriesResponse(entryCount int) (*AppendEntriesResponse, []byte) {
	resp := newAppendEntriesResponse(1, true, 1, 1)

	var buf bytes.Buffer
	resp.encode(&buf)

	return resp, buf.Bytes()
}

package wal

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/coreos/etcd/raft/raftpb"
)

func BenchmarkWrite100EntryWithoutBatch(b *testing.B) { benchmarkWriteEntry(b, 100, 0) }
func BenchmarkWrite100EntryBatch10(b *testing.B)      { benchmarkWriteEntry(b, 100, 10) }
func BenchmarkWrite100EntryBatch100(b *testing.B)     { benchmarkWriteEntry(b, 100, 100) }
func BenchmarkWrite100EntryBatch500(b *testing.B)     { benchmarkWriteEntry(b, 100, 500) }
func BenchmarkWrite100EntryBatch1000(b *testing.B)    { benchmarkWriteEntry(b, 100, 1000) }

func BenchmarkWrite1000EntryWithoutBatch(b *testing.B) { benchmarkWriteEntry(b, 1000, 0) }
func BenchmarkWrite1000EntryBatch10(b *testing.B)      { benchmarkWriteEntry(b, 1000, 10) }
func BenchmarkWrite1000EntryBatch100(b *testing.B)     { benchmarkWriteEntry(b, 1000, 100) }
func BenchmarkWrite1000EntryBatch500(b *testing.B)     { benchmarkWriteEntry(b, 1000, 500) }
func BenchmarkWrite1000EntryBatch1000(b *testing.B)    { benchmarkWriteEntry(b, 1000, 1000) }

func benchmarkWriteEntry(b *testing.B, size int, batch int) {
	p, err := ioutil.TempDir(os.TempDir(), "waltest")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(p)

	w, err := Create(p, []byte("somedata"))
	if err != nil {
		b.Fatalf("err = %v, want nil", err)
	}
	data := make([]byte, size)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i)
	}
	e := &raftpb.Entry{Data: data}

	b.ResetTimer()
	n := 0
	for i := 0; i < b.N; i++ {
		err := w.SaveEntry(e)
		if err != nil {
			b.Fatal(err)
		}
		n++
		if n > batch {
			w.sync()
			n = 0
		}
	}
}

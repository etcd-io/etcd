package test

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func BenchmarkEtcdDirectCall(b *testing.B) {
	templateBenchmarkEtcdDirectCall(b, false)
}

func BenchmarkEtcdDirectCallTls(b *testing.B) {
	templateBenchmarkEtcdDirectCall(b, true)
}

func templateBenchmarkEtcdDirectCall(b *testing.B, tls bool) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 3
	_, etcds, _ := CreateCluster(clusterSize, procAttr, tls)

	defer DestroyCluster(etcds)

	time.Sleep(time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := http.Get("http://127.0.0.1:4001/test/speed")
		resp.Body.Close()
	}
}

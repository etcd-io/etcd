package test

import (
	"net/http"
	"testing"
	"time"

	etcdtest "github.com/coreos/etcd/tests"
)

func BenchmarkEtcdDirectCall(b *testing.B) {
	templateBenchmarkEtcdDirectCall(b, false)
}

func BenchmarkEtcdDirectCallTls(b *testing.B) {
	templateBenchmarkEtcdDirectCall(b, true)
}

func templateBenchmarkEtcdDirectCall(b *testing.B, tls bool) {
	cluster := etcdtest.NewCluster(3, tls)
	cluster.Start()
	defer cluster.Stop()

	time.Sleep(time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := http.Get("http://127.0.0.1:4001/test/speed")
		resp.Body.Close()
	}
}

package itest

import (
	"github.com/coreos/etcd/client"
	"net/http"
	"testing"
	"time"
)

func Test_ITServer(t *testing.T) {

	s := NewItServer(4001)
	s.Start()

	cl, _ := client.NewHTTPClient(&http.Transport{}, "http://localhost:4001", 5*time.Second)

	_, err := cl.Get("/")
	if err != nil {
		t.Fatal("Unable to request server : %v", err)
	}

	s.Stop()
}

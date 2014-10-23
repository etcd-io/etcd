package main

import (
	"testing"

	"github.com/coreos/etcd/pkg/types"
)

func TestGenClusterString(t *testing.T) {
	tests := []struct {
		name string
		urls []string
		wstr string
	}{
		{
			"node1", []string{"http://0.0.0.0:2379", "http://1.1.1.1:2379"},
			"node1=http://0.0.0.0:2379,node1=http://1.1.1.1:2379",
		},
	}
	for i, tt := range tests {
		urls, err := types.NewURLs(tt.urls)
		if err != nil {
			t.Fatalf("unexpected new urls error: %v", err)
		}
		str := genClusterString(tt.name, urls)
		if str != tt.wstr {
			t.Errorf("#%d: cluster = %s, want %s", i, str, tt.wstr)
		}
	}
}

package embed

import (
	"net/url"
	"testing"

	"go.etcd.io/etcd/client/pkg/v3/transport"
)

func TestEmptyClientTLSInfo_createMetricsListener(t *testing.T) {
	e := &Etcd{
		cfg: Config{
			ClientTLSInfo: transport.TLSInfo{},
		},
	}

	murl := url.URL{
		Scheme: "https",
		Host:   "localhost:8080",
	}
	if _, err := e.createMetricsListener(murl); err != ErrMissingClientTLSInfoForMetricsURL {
		t.Fatalf("expected error %v, got %v", ErrMissingClientTLSInfoForMetricsURL, err)
	}
}

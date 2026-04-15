// Copyright 2026 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

func makeCert(t *testing.T, notAfter time.Time) *x509.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)

	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)

	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func tlsCtx(cert *x509.Certificate) context.Context {
	state := tls.ConnectionState{}

	if cert != nil {
		state.PeerCertificates = []*x509.Certificate{cert}
		state.VerifiedChains = [][]*x509.Certificate{{cert}}
	}
	return peer.NewContext(context.Background(), &peer.Peer{
		AuthInfo: credentials.TLSInfo{State: state},
	})
}

// histSampleCount returns the current sample count from a Prometheus histogram.
func histSampleCount(h prometheus.Histogram) uint64 {
	ch := make(chan prometheus.Metric, 1)
	h.Collect(ch)
	var m dto.Metric
	if err := (<-ch).Write(&m); err != nil || m.Histogram == nil {
		return 0
	}
	return m.Histogram.GetSampleCount()
}

// TestAuthInfoFromTLS_NoCert checks TLS peer with no certificate increments no_certificate failure count
func TestAuthInfoFromTLS_NoCert(t *testing.T) {
	as, teardown := setupAuthStore(t)
	defer teardown(t)

	before := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonNoCertificate))
	result := as.AuthInfoFromTLS(tlsCtx(nil))

	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
	if delta := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonNoCertificate)) - before; delta != 1 {
		t.Errorf("expected no_certificate counter +1. actual result: %v", delta)
	}
}

// TestAuthInfoFromTLS_MissingMetadata checks for TLS peer that missing_metadata failure count increments
func TestAuthInfoFromTLS_MissingMetadata(t *testing.T) {
	as, teardown := setupAuthStore(t)
	defer teardown(t)

	cert := makeCert(t, time.Now().Add(30*24*time.Hour))
	before := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonMissingMetadata))
	result := as.AuthInfoFromTLS(tlsCtx(cert))

	if result != nil {
		t.Errorf("expected nil. actual result: %v", result)
	}
	if delta := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonMissingMetadata)) - before; delta != 1 {
		t.Errorf("expected missing_metadata counter +1. actual result: %v", delta)
	}
}

// TestAuthInfoFromTLS_GatewayProxy checks gateway_proxy failure counter incremented.
func TestAuthInfoFromTLS_GatewayProxy(t *testing.T) {
	as, teardown := setupAuthStore(t)
	defer teardown(t)

	cert := makeCert(t, time.Now().Add(30*24*time.Hour))
	ctx := metadata.NewIncomingContext(tlsCtx(cert),
		metadata.Pairs("grpcgateway-accept", "application/json"))
	before := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonGatewayProxy))
	result := as.AuthInfoFromTLS(ctx)

	if result != nil {
		t.Errorf("expected nil. actual result: %v", result)
	}
	if delta := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonGatewayProxy)) - before; delta != 1 {
		t.Errorf("expected gateway_proxy counter +1. actual result: %v", delta)
	}
}

// TestAuthInfoFromTLS_ExpiredCert checks expired_certificate failure counter increments for TLS peer.
func TestAuthInfoFromTLS_ExpiredCert(t *testing.T) {
	as, teardown := setupAuthStore(t)
	defer teardown(t)

	cert := makeCert(t, time.Now().Add(-24*time.Hour))
	ctx := metadata.NewIncomingContext(tlsCtx(cert),
		metadata.Pairs("some-key", "some-value"))
	before := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonExpiredCertificate))
	result := as.AuthInfoFromTLS(ctx)

	if result != nil {
		t.Errorf("expected nil. actual result: %v", result)
	}
	if delta := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonExpiredCertificate)) - before; delta != 1 {
		t.Errorf("expected expired_certificate counter +1. actual result: %v", delta)
	}
}

// TestAuthInfoFromTLS_Success checks valid mTLS auth increments mtlsAuthSuccessTotal and records
// observation in expiry histogram.
func TestAuthInfoFromTLS_Success(t *testing.T) {
	as, teardown := setupAuthStore(t)
	defer teardown(t)

	cert := makeCert(t, time.Now().Add(30*24*time.Hour))
	ctx := metadata.NewIncomingContext(tlsCtx(cert), metadata.Pairs("some-key", "some-value"))

	beforeSuccess := testutil.ToFloat64(mtlsAuthSuccessTotal)
	beforeHist := histSampleCount(clientCertExpirationSecs)

	result := as.AuthInfoFromTLS(ctx)

	if result == nil || result.Username != "test-client" {
		t.Errorf("expected AuthInfo with username 'test-client'. actual result: %v", result)
	}
	if delta := testutil.ToFloat64(mtlsAuthSuccessTotal) - beforeSuccess; delta != 1 {
		t.Errorf("expected mtlsAuthSuccessTotal +1. actual result: %v", delta)
	}
	if after := histSampleCount(clientCertExpirationSecs); after <= beforeHist {
		t.Error("expected clientCertExpirationSecs to record an observation")
	}
}

// TestAuthInfoFromTLS_NoPeer checks context with no peer info returns nil.
func TestAuthInfoFromTLS_NoPeer(t *testing.T) {
	as, teardown := setupAuthStore(t)
	defer teardown(t)

	before := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonNoCertificate))
	result := as.AuthInfoFromTLS(context.Background())

	if result != nil {
		t.Errorf("expected nil. actual result: %v", result)
	}
	if delta := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonNoCertificate)) - before; delta != 0 {
		t.Errorf("no_certificate counter should not fire for missing peer. actual delta: %v", delta)
	}
}

// TestAuthInfoFromTLS_Plaintext checks plaintext client returns nil - should not increment TLS failure counters.
func TestAuthInfoFromTLS_Plaintext(t *testing.T) {
	as, teardown := setupAuthStore(t)
	defer teardown(t)

	ctx := peer.NewContext(context.Background(), &peer.Peer{})
	before := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonNoCertificate))
	result := as.AuthInfoFromTLS(ctx)

	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
	if delta := testutil.ToFloat64(mtlsAuthFailureTotal.WithLabelValues(FailReasonNoCertificate)) - before; delta != 0 {
		t.Errorf("no_certificate counter should not fire for plaintext client. actual delta: %v", delta)
	}
}

// Copyright 2016 The etcd Authors
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

package embed

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"go.etcd.io/etcd/client/pkg/v3/srv"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/featuregate"
	"go.etcd.io/etcd/server/v3/features"
)

func notFoundErr(service, domain string) error {
	name := fmt.Sprintf("_%s._tcp.%s", service, domain)
	return &net.DNSError{Err: "no such host", Name: name, Server: "10.0.0.53:53", IsTimeout: false, IsTemporary: false, IsNotFound: true}
}

func TestConfigFileOtherFields(t *testing.T) {
	ctls := securityConfig{TrustedCAFile: "cca", CertFile: "ccert", KeyFile: "ckey"}
	// Note AllowedCN and AllowedHostname are mutually exclusive, this test is just to verify the fields can be correctly marshalled & unmarshalled.
	ptls := securityConfig{TrustedCAFile: "pca", CertFile: "pcert", KeyFile: "pkey", AllowedCNs: []string{"etcd"}, AllowedHostnames: []string{"whatever.example.com"}}
	yc := struct {
		ClientSecurityCfgFile securityConfig       `json:"client-transport-security"`
		PeerSecurityCfgFile   securityConfig       `json:"peer-transport-security"`
		ForceNewCluster       bool                 `json:"force-new-cluster"`
		Logger                string               `json:"logger"`
		LogOutputs            []string             `json:"log-outputs"`
		Debug                 bool                 `json:"debug"`
		SocketOpts            transport.SocketOpts `json:"socket-options"`
	}{
		ctls,
		ptls,
		true,
		"zap",
		[]string{"/dev/null"},
		false,
		transport.SocketOpts{
			ReusePort: true,
		},
	}

	b, err := yaml.Marshal(&yc)
	if err != nil {
		t.Fatal(err)
	}

	tmpfile := mustCreateCfgFile(t, b)
	defer os.Remove(tmpfile.Name())

	cfg, err := ConfigFromFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if !ctls.equals(&cfg.ClientTLSInfo) {
		t.Errorf("ClientTLS = %v, want %v", cfg.ClientTLSInfo, ctls)
	}
	if !ptls.equals(&cfg.PeerTLSInfo) {
		t.Errorf("PeerTLS = %v, want %v", cfg.PeerTLSInfo, ptls)
	}

	assert.Truef(t, cfg.ForceNewCluster, "ForceNewCluster does not match")

	assert.Truef(t, cfg.SocketOpts.ReusePort, "ReusePort does not match")

	assert.Falsef(t, cfg.SocketOpts.ReuseAddress, "ReuseAddress does not match")
}

func TestConfigFileFeatureGates(t *testing.T) {
	testCases := []struct {
		name                   string
		serverFeatureGatesJSON string
		expectErr              bool
		expectedFeatures       map[featuregate.Feature]bool
	}{
		{
			name: "default",
			expectedFeatures: map[featuregate.Feature]bool{
				features.StopGRPCServiceOnDefrag:      false,
				features.InitialCorruptCheck:          false,
				features.TxnModeWriteWithSharedBuffer: true,
				features.LeaseCheckpoint:              false,
				features.LeaseCheckpointPersist:       false,
			},
		},
		{
			name:                   "can set feature gate StopGRPCServiceOnDefrag to true from feature gate flag",
			serverFeatureGatesJSON: "StopGRPCServiceOnDefrag=true",
			expectedFeatures: map[featuregate.Feature]bool{
				features.StopGRPCServiceOnDefrag:      true,
				features.TxnModeWriteWithSharedBuffer: true,
			},
		},
		{
			name:                   "can set feature gate InitialCorruptCheck to true from feature gate flag",
			serverFeatureGatesJSON: "InitialCorruptCheck=true",
			expectedFeatures: map[featuregate.Feature]bool{
				features.InitialCorruptCheck:          true,
				features.TxnModeWriteWithSharedBuffer: true,
			},
		},
		{
			name:                   "can set feature gate StopGRPCServiceOnDefrag to false from feature gate flag",
			serverFeatureGatesJSON: "StopGRPCServiceOnDefrag=false",
			expectedFeatures: map[featuregate.Feature]bool{
				features.StopGRPCServiceOnDefrag:      false,
				features.TxnModeWriteWithSharedBuffer: true,
			},
		},
		{
			name:                   "can set feature gate TxnModeWriteWithSharedBuffer to true from feature gate flag",
			serverFeatureGatesJSON: "TxnModeWriteWithSharedBuffer=true",
			expectedFeatures: map[featuregate.Feature]bool{
				features.TxnModeWriteWithSharedBuffer: true,
			},
		},
		{
			name:                   "can set feature gate TxnModeWriteWithSharedBuffer to false from feature gate flag",
			serverFeatureGatesJSON: "TxnModeWriteWithSharedBuffer=false",
			expectedFeatures: map[featuregate.Feature]bool{
				features.TxnModeWriteWithSharedBuffer: false,
			},
		},
		{
			name:                   "can set feature gate CompactHashCheck to true from feature gate flag",
			serverFeatureGatesJSON: "CompactHashCheck=true",
			expectedFeatures: map[featuregate.Feature]bool{
				features.CompactHashCheck:             true,
				features.TxnModeWriteWithSharedBuffer: true,
			},
		},
		{
			name:                   "can set feature gate LeaseCheckpoint and LeaseCheckpointPersist to true from feature gate flag",
			serverFeatureGatesJSON: "LeaseCheckpointPersist=true,LeaseCheckpoint=true",
			expectedFeatures: map[featuregate.Feature]bool{
				features.TxnModeWriteWithSharedBuffer: true,
				features.LeaseCheckpoint:              true,
				features.LeaseCheckpointPersist:       true,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			yc := struct {
				ServerFeatureGatesJSON string `json:"feature-gates"`
			}{
				ServerFeatureGatesJSON: tc.serverFeatureGatesJSON,
			}

			b, err := yaml.Marshal(&yc)
			if err != nil {
				t.Fatal(err)
			}

			tmpfile := mustCreateCfgFile(t, b)
			defer os.Remove(tmpfile.Name())

			cfg, err := ConfigFromFile(tmpfile.Name())
			if tc.expectErr {
				require.Errorf(t, err, "expect parse error")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for f := range features.DefaultEtcdServerFeatureGates {
				if tc.expectedFeatures[f] != cfg.ServerFeatureGate.Enabled(f) {
					t.Errorf("expected feature gate %s=%v, got %v", f, tc.expectedFeatures[f], cfg.ServerFeatureGate.Enabled(f))
				}
			}
		})
	}
}

// TestUpdateDefaultClusterFromName ensures that etcd can start with 'etcd --name=abc'.
func TestUpdateDefaultClusterFromName(t *testing.T) {
	cfg := NewConfig()
	defaultInitialCluster := cfg.InitialCluster
	oldscheme := cfg.AdvertisePeerUrls[0].Scheme
	origpeer := cfg.AdvertisePeerUrls[0].String()
	origadvc := cfg.AdvertiseClientUrls[0].String()

	cfg.Name = "abc"
	lpport := cfg.ListenPeerUrls[0].Port()

	// in case of 'etcd --name=abc'
	exp := fmt.Sprintf("%s=%s://localhost:%s", cfg.Name, oldscheme, lpport)
	_, _ = cfg.UpdateDefaultClusterFromName(defaultInitialCluster)
	require.Equalf(t, exp, cfg.InitialCluster, "initial-cluster expected %q, got %q", exp, cfg.InitialCluster)
	// advertise peer URL should not be affected
	require.Equalf(t, origpeer, cfg.AdvertisePeerUrls[0].String(), "advertise peer url expected %q, got %q", origadvc, cfg.AdvertisePeerUrls[0].String())
	// advertise client URL should not be affected
	require.Equalf(t, origadvc, cfg.AdvertiseClientUrls[0].String(), "advertise client url expected %q, got %q", origadvc, cfg.AdvertiseClientUrls[0].String())
}

// TestUpdateDefaultClusterFromNameOverwrite ensures that machine's default host is only used
// if advertise URLs are default values(localhost:2379,2380) AND if listen URL is 0.0.0.0.
func TestUpdateDefaultClusterFromNameOverwrite(t *testing.T) {
	if defaultHostname == "" {
		t.Skip("machine's default host not found")
	}

	cfg := NewConfig()
	defaultInitialCluster := cfg.InitialCluster
	oldscheme := cfg.AdvertisePeerUrls[0].Scheme
	origadvc := cfg.AdvertiseClientUrls[0].String()

	cfg.Name = "abc"
	lpport := cfg.ListenPeerUrls[0].Port()
	cfg.ListenPeerUrls[0] = url.URL{Scheme: cfg.ListenPeerUrls[0].Scheme, Host: fmt.Sprintf("0.0.0.0:%s", lpport)}
	dhost, _ := cfg.UpdateDefaultClusterFromName(defaultInitialCluster)
	require.Equalf(t, dhost, defaultHostname, "expected default host %q, got %q", defaultHostname, dhost)
	aphost, apport := cfg.AdvertisePeerUrls[0].Hostname(), cfg.AdvertisePeerUrls[0].Port()
	require.Equalf(t, apport, lpport, "advertise peer url got different port %s, expected %s", apport, lpport)
	require.Equalf(t, aphost, defaultHostname, "advertise peer url expected machine default host %q, got %q", defaultHostname, aphost)
	expected := fmt.Sprintf("%s=%s://%s:%s", cfg.Name, oldscheme, defaultHostname, lpport)
	require.Equalf(t, expected, cfg.InitialCluster, "initial-cluster expected %q, got %q", expected, cfg.InitialCluster)

	// advertise client URL should not be affected
	require.Equalf(t, origadvc, cfg.AdvertiseClientUrls[0].String(), "advertise-client-url expected %q, got %q", origadvc, cfg.AdvertiseClientUrls[0].String())
}

func TestInferLocalAddr(t *testing.T) {
	tests := []struct {
		name               string
		advertisePeerURLs  []string
		serverFeatureGates string
		expectedLocalAddr  string
	}{
		{
			"defaults, SetMemberLocalAddr=false ",
			[]string{DefaultInitialAdvertisePeerURLs},
			"SetMemberLocalAddr=false",
			"",
		},
		{
			"IPv4 address, SetMemberLocalAddr=false ",
			[]string{"https://192.168.100.110:2380"},
			"SetMemberLocalAddr=false",
			"",
		},
		{
			"defaults, SetMemberLocalAddr=true",
			[]string{DefaultInitialAdvertisePeerURLs},
			"SetMemberLocalAddr=true",
			"",
		},
		{
			"IPv4 unspecified address, SetMemberLocalAddr=true",
			[]string{"https://0.0.0.0:2380"},
			"SetMemberLocalAddr=true",
			"",
		},
		{
			"IPv6 unspecified address, SetMemberLocalAddr=true",
			[]string{"https://[::]:2380"},
			"SetMemberLocalAddr=true",
			"",
		},
		{
			"IPv4 loopback address, SetMemberLocalAddr=true",
			[]string{"https://127.0.0.1:2380"},
			"SetMemberLocalAddr=true",
			"",
		},
		{
			"IPv6 loopback address, SetMemberLocalAddr=true",
			[]string{"https://[::1]:2380"},
			"SetMemberLocalAddr=true",
			"",
		},
		{
			"IPv4 address, SetMemberLocalAddr=true",
			[]string{"https://192.168.100.110:2380"},
			"SetMemberLocalAddr=true",
			"192.168.100.110",
		},
		{
			"Hostname only, SetMemberLocalAddr=true",
			[]string{"https://123-host-3.corp.internal:2380"},
			"SetMemberLocalAddr=true",
			"",
		},
		{
			"Hostname and IPv4 address, SetMemberLocalAddr=true",
			[]string{"https://123-host-3.corp.internal:2380", "https://192.168.100.110:2380"},
			"SetMemberLocalAddr=true",
			"192.168.100.110",
		},
		{
			"IPv4 address and Hostname, SetMemberLocalAddr=true",
			[]string{"https://192.168.100.110:2380", "https://123-host-3.corp.internal:2380"},
			"SetMemberLocalAddr=true",
			"192.168.100.110",
		},
		{
			"IPv4 and IPv6 addresses, SetMemberLocalAddr=true",
			[]string{"https://192.168.100.110:2380", "https://[2001:db8:85a3::8a2e:370:7334]:2380"},
			"SetMemberLocalAddr=true",
			"192.168.100.110",
		},
		{
			"IPv6 and IPv4 addresses, SetMemberLocalAddr=true",
			// IPv4 addresses will always sort before IPv6 ones anyway
			[]string{"https://[2001:db8:85a3::8a2e:370:7334]:2380", "https://192.168.100.110:2380"},
			"SetMemberLocalAddr=true",
			"192.168.100.110",
		},
		{
			"Hostname, IPv4 and IPv6 addresses, SetMemberLocalAddr=true",
			[]string{"https://123-host-3.corp.internal:2380", "https://192.168.100.110:2380", "https://[2001:db8:85a3::8a2e:370:7334]:2380"},
			"SetMemberLocalAddr=true",
			"192.168.100.110",
		},
		{
			"Hostname, IPv6 and IPv4 addresses, SetMemberLocalAddr=true",
			// IPv4 addresses will always sort before IPv6 ones anyway
			[]string{"https://123-host-3.corp.internal:2380", "https://[2001:db8:85a3::8a2e:370:7334]:2380", "https://192.168.100.110:2380"},
			"SetMemberLocalAddr=true",
			"192.168.100.110",
		},
		{
			"IPv6 address, SetMemberLocalAddr=true",
			[]string{"https://[2001:db8:85a3::8a2e:370:7334]:2380"},
			"SetMemberLocalAddr=true",
			"2001:db8:85a3::8a2e:370:7334",
		},
		{
			"Hostname and IPv6 address, SetMemberLocalAddr=true",
			[]string{"https://123-host-3.corp.internal:2380", "https://[2001:db8:85a3::8a2e:370:7334]:2380"},
			"SetMemberLocalAddr=true",
			"2001:db8:85a3::8a2e:370:7334",
		},
		{
			"IPv6 address and Hostname, SetMemberLocalAddr=true",
			[]string{"https://[2001:db8:85a3::8a2e:370:7334]:2380", "https://123-host-3.corp.internal:2380"},
			"SetMemberLocalAddr=true",
			"2001:db8:85a3::8a2e:370:7334",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.AdvertisePeerUrls = types.MustNewURLs(tt.advertisePeerURLs)
			cfg.ServerFeatureGate.(featuregate.MutableFeatureGate).Set(tt.serverFeatureGates)

			require.NoError(t, cfg.Validate())
			require.Equal(t, tt.expectedLocalAddr, cfg.InferLocalAddr())
		})
	}
}

func TestSetMemberLocalAddrValidate(t *testing.T) {
	tcs := []struct {
		name               string
		serverFeatureGates string
	}{
		{
			name: "Default config should pass",
		},
		{
			name:               "Enabling SetMemberLocalAddr should pass",
			serverFeatureGates: "SetMemberLocalAddr=true",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			cfg := *NewConfig()
			cfg.ServerFeatureGate.(featuregate.MutableFeatureGate).Set(tc.serverFeatureGates)
			err := cfg.Validate()
			require.NoError(t, err)
		})
	}
}

func (s *securityConfig) equals(t *transport.TLSInfo) bool {
	return s.CertFile == t.CertFile &&
		s.CertAuth == t.ClientCertAuth &&
		s.TrustedCAFile == t.TrustedCAFile &&
		s.ClientCertFile == t.ClientCertFile &&
		s.ClientKeyFile == t.ClientKeyFile &&
		s.KeyFile == t.KeyFile &&
		compareSlices(s.AllowedCNs, t.AllowedCNs) &&
		compareSlices(s.AllowedHostnames, t.AllowedHostnames)
}

func compareSlices(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	for i, v := range slice1 {
		if v != slice2[i] {
			return false
		}
	}
	return true
}

func mustCreateCfgFile(t *testing.T, b []byte) *os.File {
	tmpfile, err := os.CreateTemp(t.TempDir(), "servercfg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tmpfile.Write(b); err != nil {
		t.Fatal(err)
	}
	if err = tmpfile.Close(); err != nil {
		t.Fatal(err)
	}
	return tmpfile
}

func TestAutoCompactionModeInvalid(t *testing.T) {
	cfg := NewConfig()
	cfg.Logger = "zap"
	cfg.LogOutputs = []string{"/dev/null"}
	cfg.AutoCompactionMode = "period"
	err := cfg.Validate()
	if err == nil {
		t.Errorf("expected non-nil error, got %v", err)
	}
}

func TestAutoCompactionModeParse(t *testing.T) {
	tests := []struct {
		mode      string
		retention string
		werr      bool
		wdur      time.Duration
	}{
		// revision
		{"revision", "1", false, 1},
		{"revision", "1h", false, time.Hour},
		{"revision", "a", true, 0},
		{"revision", "-1", true, 0},
		// periodic
		{"periodic", "1", false, time.Hour},
		{"periodic", "a", true, 0},
		{"revision", "-1", true, 0},
		// err mode
		{"errmode", "1", false, 0},
		{"errmode", "1h", false, time.Hour},
		// empty mode
		{"", "1", true, 0},
		{"", "1h", false, time.Hour},
		{"", "a", true, 0},
		{"", "-1", true, 0},
	}

	hasErr := func(err error) bool {
		return err != nil
	}

	for i, tt := range tests {
		dur, err := parseCompactionRetention(tt.mode, tt.retention)
		if hasErr(err) != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if dur != tt.wdur {
			t.Errorf("#%d: duration = %s, want %s", i, dur, tt.wdur)
		}
	}
}

func TestPeerURLsMapAndTokenFromSRV(t *testing.T) {
	defer func() { getCluster = srv.GetCluster }()

	tests := []struct {
		withSSL    []string
		withoutSSL []string
		apurls     []string
		wurls      string
		werr       bool
	}{
		{
			[]string{},
			[]string{},
			[]string{"http://localhost:2380"},
			"",
			true,
		},
		{
			[]string{"1.example.com=https://1.example.com:2380", "0=https://2.example.com:2380", "1=https://3.example.com:2380"},
			[]string{},
			[]string{"https://1.example.com:2380"},
			"0=https://2.example.com:2380,1.example.com=https://1.example.com:2380,1=https://3.example.com:2380",
			false,
		},
		{
			[]string{"1.example.com=https://1.example.com:2380"},
			[]string{"0=http://2.example.com:2380", "1=http://3.example.com:2380"},
			[]string{"https://1.example.com:2380"},
			"0=http://2.example.com:2380,1.example.com=https://1.example.com:2380,1=http://3.example.com:2380",
			false,
		},
		{
			[]string{},
			[]string{"1.example.com=http://1.example.com:2380", "0=http://2.example.com:2380", "1=http://3.example.com:2380"},
			[]string{"http://1.example.com:2380"},
			"0=http://2.example.com:2380,1.example.com=http://1.example.com:2380,1=http://3.example.com:2380",
			false,
		},
	}

	hasErr := func(err error) bool {
		return err != nil
	}

	for i, tt := range tests {
		getCluster = func(serviceScheme string, service string, name string, dns string, apurls types.URLs) ([]string, error) {
			var urls []string
			if serviceScheme == "https" && service == "etcd-server-ssl" {
				urls = tt.withSSL
			} else if serviceScheme == "http" && service == "etcd-server" {
				urls = tt.withoutSSL
			}
			if len(urls) > 0 {
				return urls, nil
			}
			return urls, notFoundErr(service, dns)
		}

		cfg := NewConfig()
		cfg.Name = "1.example.com"
		cfg.InitialCluster = ""
		cfg.InitialClusterToken = ""
		cfg.DNSCluster = "example.com"
		cfg.AdvertisePeerUrls = types.MustNewURLs(tt.apurls)

		if err := cfg.Validate(); err != nil {
			t.Errorf("#%d: failed to validate test Config: %v", i, err)
			continue
		}

		urlsmap, _, err := cfg.PeerURLsMapAndToken("etcd")
		if urlsmap.String() != tt.wurls {
			t.Errorf("#%d: urlsmap = %s, want = %s", i, urlsmap.String(), tt.wurls)
		}
		if hasErr(err) != tt.werr {
			t.Errorf("#%d: err = %v, want = %v", i, err, tt.werr)
		}
	}
}

func TestLeaseCheckpointValidate(t *testing.T) {
	tcs := []struct {
		name               string
		serverFeatureGates string
		expectError        bool
	}{
		{
			name: "Default config should pass",
		},
		{
			name:               "Enabling checkpoint leases should pass",
			serverFeatureGates: "LeaseCheckpoint=true",
		},
		{
			name:               "Enabling checkpoint leases and persist should pass",
			serverFeatureGates: "LeaseCheckpointPersist=true,LeaseCheckpoint=true",
		},
		{
			name:               "Enabling checkpoint leases persist without checkpointing itself should fail",
			serverFeatureGates: "LeaseCheckpointPersist=true",
			expectError:        true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			cfg := *NewConfig()
			cfg.ServerFeatureGate.(featuregate.MutableFeatureGate).Set(tc.serverFeatureGates)
			err := cfg.Validate()
			if (err != nil) != tc.expectError {
				t.Errorf("config.Validate() = %q, expected error: %v", err, tc.expectError)
			}
		})
	}
}

func TestLogRotation(t *testing.T) {
	tests := []struct {
		name              string
		logOutputs        []string
		logRotationConfig string
		wantErr           bool
		wantErrMsg        error
	}{
		{
			name:              "mixed log output targets",
			logOutputs:        []string{"stderr", "/tmp/path"},
			logRotationConfig: `{"maxsize": 1}`,
		},
		{
			name:              "log output relative path",
			logOutputs:        []string{"stderr", "tmp/path"},
			logRotationConfig: `{"maxsize": 1}`,
		},
		{
			name:              "no file targets",
			logOutputs:        []string{"stderr"},
			logRotationConfig: `{"maxsize": 1}`,
			wantErr:           true,
			wantErrMsg:        ErrLogRotationInvalidLogOutput,
		},
		{
			name:              "multiple file targets",
			logOutputs:        []string{"/tmp/path1", "/tmp/path2"},
			logRotationConfig: DefaultLogRotationConfig,
			wantErr:           true,
			wantErrMsg:        ErrLogRotationInvalidLogOutput,
		},
		{
			name:              "default output",
			logRotationConfig: `{"maxsize": 1}`,
			wantErr:           true,
			wantErrMsg:        ErrLogRotationInvalidLogOutput,
		},
		{
			name:              "default log rotation config",
			logOutputs:        []string{"/tmp/path"},
			logRotationConfig: DefaultLogRotationConfig,
		},
		{
			name:              "invalid logger config",
			logOutputs:        []string{"/tmp/path"},
			logRotationConfig: `{"maxsize": true}`,
			wantErr:           true,
			wantErrMsg:        errors.New("invalid log rotation config: json: cannot unmarshal bool into Go struct field logRotationConfig.Logger.maxsize of type int"),
		},
		{
			name:              "improperly formatted logger config",
			logOutputs:        []string{"/tmp/path"},
			logRotationConfig: `{"maxsize": true`,
			wantErr:           true,
			wantErrMsg:        errors.New("improperly formatted log rotation config: unexpected end of JSON input"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.Logger = "zap"
			cfg.LogOutputs = tt.logOutputs
			cfg.EnableLogRotation = true
			cfg.LogRotationConfigJSON = tt.logRotationConfig
			err := cfg.Validate()
			if err != nil && !tt.wantErr {
				t.Errorf("test %q, unexpected error %v", tt.name, err)
			}
			if err != nil && tt.wantErr && tt.wantErrMsg.Error() != err.Error() {
				t.Errorf("test %q, expected error: %+v, got: %+v", tt.name, tt.wantErrMsg, err)
			}
			if err == nil && tt.wantErr {
				t.Errorf("test %q, expected error, got nil", tt.name)
			}
			if err == nil {
				cfg.GetLogger().Info("test log")
			}
		})
	}
}

func TestTLSVersionMinMax(t *testing.T) {
	tests := []struct {
		name                  string
		givenTLSMinVersion    string
		givenTLSMaxVersion    string
		givenCipherSuites     []string
		expectError           bool
		expectedMinTLSVersion uint16
		expectedMaxTLSVersion uint16
	}{
		{
			name:                  "Minimum TLS version is set",
			givenTLSMinVersion:    "TLS1.3",
			expectedMinTLSVersion: tls.VersionTLS13,
			expectedMaxTLSVersion: 0,
		},
		{
			name:                  "Maximum TLS version is set",
			givenTLSMaxVersion:    "TLS1.2",
			expectedMinTLSVersion: 0,
			expectedMaxTLSVersion: tls.VersionTLS12,
		},
		{
			name:                  "Minimum and Maximum TLS versions are set",
			givenTLSMinVersion:    "TLS1.3",
			givenTLSMaxVersion:    "TLS1.3",
			expectedMinTLSVersion: tls.VersionTLS13,
			expectedMaxTLSVersion: tls.VersionTLS13,
		},
		{
			name:               "Minimum and Maximum TLS versions are set in reverse order",
			givenTLSMinVersion: "TLS1.3",
			givenTLSMaxVersion: "TLS1.2",
			expectError:        true,
		},
		{
			name:               "Invalid minimum TLS version",
			givenTLSMinVersion: "invalid version",
			expectError:        true,
		},
		{
			name:               "Invalid maximum TLS version",
			givenTLSMaxVersion: "invalid version",
			expectError:        true,
		},
		{
			name:               "Cipher suites configured for TLS 1.3",
			givenTLSMinVersion: "TLS1.3",
			givenCipherSuites:  []string{"TLS_AES_128_GCM_SHA256"},
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.TlsMinVersion = tt.givenTLSMinVersion
			cfg.TlsMaxVersion = tt.givenTLSMaxVersion
			cfg.CipherSuites = tt.givenCipherSuites

			err := cfg.Validate()
			if err != nil {
				assert.Truef(t, tt.expectError, "Validate() returned error while expecting success: %v", err)
				return
			}

			updateMinMaxVersions(&cfg.PeerTLSInfo, cfg.TlsMinVersion, cfg.TlsMaxVersion)
			updateMinMaxVersions(&cfg.ClientTLSInfo, cfg.TlsMinVersion, cfg.TlsMaxVersion)

			assert.Equal(t, tt.expectedMinTLSVersion, cfg.PeerTLSInfo.MinVersion)
			assert.Equal(t, tt.expectedMaxTLSVersion, cfg.PeerTLSInfo.MaxVersion)
			assert.Equal(t, tt.expectedMinTLSVersion, cfg.ClientTLSInfo.MinVersion)
			assert.Equal(t, tt.expectedMaxTLSVersion, cfg.ClientTLSInfo.MaxVersion)
		})
	}
}

func TestUndefinedAutoCompactionModeValidate(t *testing.T) {
	cfg := *NewConfig()
	cfg.AutoCompactionMode = ""
	err := cfg.Validate()
	require.Error(t, err)
}

func TestMatchNewConfigAddFlags(t *testing.T) {
	cfg := NewConfig()
	fs := flag.NewFlagSet("etcd", flag.ContinueOnError)
	cfg.AddFlags(fs)
	require.NoError(t, fs.Parse(nil))
	// TODO: Reduce number of unexported fields set in config
	if diff := cmp.Diff(NewConfig(), cfg, cmpopts.IgnoreUnexported(transport.TLSInfo{}, Config{}), cmp.Comparer(func(a, b featuregate.FeatureGate) bool {
		return a.String() == b.String()
	})); diff != "" {
		t.Errorf("Diff: %s", diff)
	}
}

type CertificateSubject struct {
	Organization  []string
	Country       []string
	Province      []string
	Locality      []string
	StreetAddress []string
	PostalCode    []string
}

func generateCACert(t *testing.T, subject *CertificateSubject) (*x509.Certificate, *rsa.PrivateKey) {
	caCert := &x509.Certificate{
		IsCA:         true,
		SerialNumber: big.NewInt(2023),
		Subject: pkix.Name{
			Organization:  subject.Organization,
			Country:       subject.Country,
			Province:      subject.Province,
			Locality:      subject.Locality,
			StreetAddress: subject.StreetAddress,
			PostalCode:    subject.PostalCode,
		},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 1),
		BasicConstraintsValid: true,
	}

	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)

	caBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, &caPrivateKey.PublicKey, caPrivateKey)
	require.NoError(t, err)

	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivateKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivateKey),
	})

	return caCert, caPrivateKey
}

func generateHostCertificateFromCA(t *testing.T, caCert *x509.Certificate, caPrivateKey *rsa.PrivateKey, subject *CertificateSubject) tls.Certificate {
	tlsCertSpecification := &x509.Certificate{
		IsCA:         false,
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  subject.Organization,
			Country:       subject.Country,
			Province:      subject.Province,
			Locality:      subject.Locality,
			StreetAddress: subject.StreetAddress,
			PostalCode:    subject.PostalCode,
		},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)

	certBytes, err := x509.CreateCertificate(rand.Reader, tlsCertSpecification, caCert, &certPrivKey.PublicKey, caPrivateKey)
	require.NoError(t, err)

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	serverCert, err := tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
	require.NoError(t, err)

	return serverCert
}

func TestUpdateCipherSuiteWithTLSConfig(t *testing.T) {
	t.Parallel()

	caSubject := &CertificateSubject{
		Organization:  []string{"Company, Testing"},
		Country:       []string{"Test"},
		Province:      []string{"Test Province"},
		Locality:      []string{"Testing"},
		StreetAddress: []string{"Test Street"},
		PostalCode:    []string{"01234"},
	}
	caCert, caPrivateKey := generateCACert(t, caSubject)

	serverSubject := &CertificateSubject{
		Organization:  []string{"Company, Testing"},
		Country:       []string{"Test"},
		Province:      []string{"Test Province"},
		Locality:      []string{"Testing"},
		StreetAddress: []string{"Test Street"},
		PostalCode:    []string{"01234"},
	}
	serverCert := generateHostCertificateFromCA(t, caCert, caPrivateKey, serverSubject)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}

	cfg := NewConfig()
	cfg.CustomClientTLSConfig = tlsConfig

	err := updateCipherSuites(tlsConfig, []string{"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", "TLS_RSA_WITH_AES_128_CBC_SHA"})
	if assert.Error(t, err) {
		assert.Equal(t, fmt.Errorf("tlsConfig.CipherSuites should not be updated, the default is already secure"), err)
	}
}

func TestMinMaxVersion(t *testing.T) {
	t.Parallel()

	caSubject := &CertificateSubject{
		Organization:  []string{"Company, Testing"},
		Country:       []string{"Test"},
		Province:      []string{"Test Province"},
		Locality:      []string{"Testing"},
		StreetAddress: []string{"Test Street"},
		PostalCode:    []string{"01234"},
	}
	caCert, caPrivateKey := generateCACert(t, caSubject)

	serverSubject := &CertificateSubject{
		Organization:  []string{"Company, Testing"},
		Country:       []string{"Test"},
		Province:      []string{"Test Province"},
		Locality:      []string{"Testing"},
		StreetAddress: []string{"Test Street"},
		PostalCode:    []string{"01234"},
	}
	serverCert := generateHostCertificateFromCA(t, caCert, caPrivateKey, serverSubject)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}

	cfg := NewConfig()
	cfg.CustomClientTLSConfig = tlsConfig

	expMinVersion := uint16(0x303)
	expMaxVersion := uint16(0x304)
	updateMinMaxVersions(tlsConfig, "TLS1.2", "TLS1.3")
	assert.Equal(t, expMinVersion, tlsConfig.MinVersion)
	assert.Equal(t, expMaxVersion, tlsConfig.MaxVersion)
}

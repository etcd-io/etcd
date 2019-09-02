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
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"testing"
	"time"

	"go.etcd.io/etcd/pkg/transport"

	"sigs.k8s.io/yaml"
)

func TestConfigFileOtherFields(t *testing.T) {
	ctls := securityConfig{TrustedCAFile: "cca", CertFile: "ccert", KeyFile: "ckey"}
	ptls := securityConfig{TrustedCAFile: "pca", CertFile: "pcert", KeyFile: "pkey"}
	yc := struct {
		ClientSecurityCfgFile securityConfig `json:"client-transport-security"`
		PeerSecurityCfgFile   securityConfig `json:"peer-transport-security"`
		ForceNewCluster       bool           `json:"force-new-cluster"`
		Logger                string         `json:"logger"`
		LogOutputs            []string       `json:"log-outputs"`
		Debug                 bool           `json:"debug"`
	}{
		ctls,
		ptls,
		true,
		"zap",
		[]string{"/dev/null"},
		false,
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

	if !cfg.ForceNewCluster {
		t.Errorf("ForceNewCluster = %v, want %v", cfg.ForceNewCluster, true)
	}

	if !ctls.equals(&cfg.ClientTLSInfo) {
		t.Errorf("ClientTLS = %v, want %v", cfg.ClientTLSInfo, ctls)
	}
	if !ptls.equals(&cfg.PeerTLSInfo) {
		t.Errorf("PeerTLS = %v, want %v", cfg.PeerTLSInfo, ptls)
	}
}

// TestUpdateDefaultClusterFromName ensures that etcd can start with 'etcd --name=abc'.
func TestUpdateDefaultClusterFromName(t *testing.T) {
	cfg := NewConfig()
	defaultInitialCluster := cfg.InitialCluster
	oldscheme := cfg.APUrls[0].Scheme
	origpeer := cfg.APUrls[0].String()
	origadvc := cfg.ACUrls[0].String()

	cfg.Name = "abc"
	lpport := cfg.LPUrls[0].Port()

	// in case of 'etcd --name=abc'
	exp := fmt.Sprintf("%s=%s://localhost:%s", cfg.Name, oldscheme, lpport)
	cfg.UpdateDefaultClusterFromName(defaultInitialCluster)
	if exp != cfg.InitialCluster {
		t.Fatalf("initial-cluster expected %q, got %q", exp, cfg.InitialCluster)
	}
	// advertise peer URL should not be affected
	if origpeer != cfg.APUrls[0].String() {
		t.Fatalf("advertise peer url expected %q, got %q", origadvc, cfg.APUrls[0].String())
	}
	// advertise client URL should not be affected
	if origadvc != cfg.ACUrls[0].String() {
		t.Fatalf("advertise client url expected %q, got %q", origadvc, cfg.ACUrls[0].String())
	}
}

// TestUpdateDefaultClusterFromNameOverwrite ensures that machine's default host is only used
// if advertise URLs are default values(localhost:2379,2380) AND if listen URL is 0.0.0.0.
func TestUpdateDefaultClusterFromNameOverwrite(t *testing.T) {
	if defaultHostname == "" {
		t.Skip("machine's default host not found")
	}

	cfg := NewConfig()
	defaultInitialCluster := cfg.InitialCluster
	oldscheme := cfg.APUrls[0].Scheme
	origadvc := cfg.ACUrls[0].String()

	cfg.Name = "abc"
	lpport := cfg.LPUrls[0].Port()
	cfg.LPUrls[0] = url.URL{Scheme: cfg.LPUrls[0].Scheme, Host: fmt.Sprintf("0.0.0.0:%s", lpport)}
	dhost, _ := cfg.UpdateDefaultClusterFromName(defaultInitialCluster)
	if dhost != defaultHostname {
		t.Fatalf("expected default host %q, got %q", defaultHostname, dhost)
	}
	aphost, apport := cfg.APUrls[0].Hostname(), cfg.APUrls[0].Port()
	if apport != lpport {
		t.Fatalf("advertise peer url got different port %s, expected %s", apport, lpport)
	}
	if aphost != defaultHostname {
		t.Fatalf("advertise peer url expected machine default host %q, got %q", defaultHostname, aphost)
	}
	expected := fmt.Sprintf("%s=%s://%s:%s", cfg.Name, oldscheme, defaultHostname, lpport)
	if expected != cfg.InitialCluster {
		t.Fatalf("initial-cluster expected %q, got %q", expected, cfg.InitialCluster)
	}

	// advertise client URL should not be affected
	if origadvc != cfg.ACUrls[0].String() {
		t.Fatalf("advertise-client-url expected %q, got %q", origadvc, cfg.ACUrls[0].String())
	}
}

func (s *securityConfig) equals(t *transport.TLSInfo) bool {
	return s.CertFile == t.CertFile &&
		s.CertAuth == t.ClientCertAuth &&
		s.TrustedCAFile == t.TrustedCAFile
}

func mustCreateCfgFile(t *testing.T, b []byte) *os.File {
	tmpfile, err := ioutil.TempFile("", "servercfg")
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
	cfg.Debug = false
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
		// periodic
		{"periodic", "1", false, time.Hour},
		{"periodic", "a", true, 0},
		// err mode
		{"errmode", "1", false, 0},
		{"errmode", "1h", false, time.Hour},
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

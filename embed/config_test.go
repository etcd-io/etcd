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
	"io/ioutil"
	"os"
	"testing"

	"github.com/coreos/etcd/pkg/transport"
	"github.com/ghodss/yaml"
)

func TestConfigFileOtherFields(t *testing.T) {
	ctls := securityConfig{CAFile: "cca", CertFile: "ccert", KeyFile: "ckey"}
	ptls := securityConfig{CAFile: "pca", CertFile: "pcert", KeyFile: "pkey"}
	yc := struct {
		ClientSecurityCfgFile securityConfig `json:"client-transport-security"`
		PeerSecurityCfgFile   securityConfig `json:"peer-transport-security"`
		ForceNewCluster       bool           `json:"force-new-cluster"`
	}{
		ctls,
		ptls,
		true,
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

func (s *securityConfig) equals(t *transport.TLSInfo) bool {
	return s.CAFile == t.CAFile &&
		s.CertFile == t.CertFile &&
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

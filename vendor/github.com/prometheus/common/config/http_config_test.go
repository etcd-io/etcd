// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

var invalidHTTPClientConfigs = []struct {
	httpClientConfigFile string
	errMsg               string
}{
	{
		httpClientConfigFile: "testdata/http.conf.bearer-token-and-file-set.bad.yml",
		errMsg:               "at most one of bearer_token & bearer_token_file must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.empty.bad.yml",
		errMsg:               "at most one of basic_auth, bearer_token & bearer_token_file must be configured",
	},
}

func TestAuthRoundTrippers(t *testing.T) {

	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.good.yml")
	if err != nil {
		t.Errorf("Error loading HTTP client config: %v", err)
	}

	tlsConfig, err := NewTLSConfig(&cfg.TLSConfig)
	if err != nil {
		t.Errorf("Error creating new TLS config: %v", err)
	}

	rt := &http.Transport{
		Proxy:             http.ProxyURL(cfg.ProxyURL.URL),
		DisableKeepAlives: true,
		TLSClientConfig:   tlsConfig,
	}
	req := new(http.Request)

	bearerAuthRoundTripper := NewBearerAuthRoundTripper("mysecret", rt)
	bearerAuthRoundTripper.RoundTrip(req)

	basicAuthRoundTripper := NewBasicAuthRoundTripper("username", "password", rt)
	basicAuthRoundTripper.RoundTrip(req)
}

func TestHideHTTPClientConfigSecrets(t *testing.T) {
	c, _, err := LoadHTTPConfigFile("testdata/http.conf.good.yml")
	if err != nil {
		t.Errorf("Error parsing %s: %s", "testdata/http.conf.good.yml", err)
	}

	// String method must not reveal authentication credentials.
	s := c.String()
	if strings.Contains(s, "mysecret") {
		t.Fatal("http client config's String method reveals authentication credentials.")
	}
}

func mustParseURL(u string) *URL {
	parsed, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	return &URL{URL: parsed}
}

func TestNewClientFromConfig(t *testing.T) {
	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.good.yml")
	if err != nil {
		t.Errorf("Error loading HTTP client config: %v", err)
	}
	_, err = NewHTTPClientFromConfig(cfg)
	if err != nil {
		t.Errorf("Error creating new client from config: %v", err)
	}
}

func TestNewClientFromInvalidConfig(t *testing.T) {
	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.invalid-bearer-token-file.bad.yml")
	if err != nil {
		t.Errorf("Error loading HTTP client config: %v", err)
	}
	_, err = NewHTTPClientFromConfig(cfg)
	if err == nil {
		t.Error("Expected error creating new client from invalid config but got none")
	}
	if !strings.Contains(err.Error(), "unable to read bearer token file file: open file: no such file or directory") {
		t.Errorf("Expected error with config but got: %s", err.Error())
	}
}

func TestValidateHTTPConfig(t *testing.T) {
	cfg, _, err := LoadHTTPConfigFile("testdata/http.conf.good.yml")
	if err != nil {
		t.Errorf("Error loading HTTP client config: %v", err)
	}
	err = cfg.validate()
	if err != nil {
		t.Fatalf("Error validating %s: %s", "testdata/http.conf.good.yml", err)
	}
}

func TestInvalidHTTPConfigs(t *testing.T) {
	for _, ee := range invalidHTTPClientConfigs {
		_, _, err := LoadHTTPConfigFile(ee.httpClientConfigFile)
		if err == nil {
			t.Error("Expected error with config but got none")
			continue
		}
		if !strings.Contains(err.Error(), ee.errMsg) {
			t.Errorf("Expected error for invalid HTTP client configuration to contain %q but got: %s", ee.errMsg, err)
		}
	}
}

// LoadHTTPConfig parses the YAML input s into a HTTPClientConfig.
func LoadHTTPConfig(s string) (*HTTPClientConfig, error) {
	cfg := &HTTPClientConfig{}
	err := yaml.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadHTTPConfigFile parses the given YAML file into a HTTPClientConfig.
func LoadHTTPConfigFile(filename string) (*HTTPClientConfig, []byte, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := LoadHTTPConfig(string(content))
	if err != nil {
		return nil, nil, err
	}
	return cfg, content, nil
}

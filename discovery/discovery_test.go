// Copyright 2015 CoreOS, Inc.
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

package discovery

import (
	"net/http"
	"testing"
)

func TestNewProxyFuncUnset(t *testing.T) {
	pf, err := newProxyFunc("")
	if pf != nil {
		t.Fatal("unexpected non-nil proxyFunc")
	}
	if err != nil {
		t.Fatalf("unexpected non-nil err: %v", err)
	}
}

func TestNewProxyFuncBad(t *testing.T) {
	tests := []string{
		"%%",
		"http://foo.com/%1",
	}
	for i, in := range tests {
		pf, err := newProxyFunc(in)
		if pf != nil {
			t.Errorf("#%d: unexpected non-nil proxyFunc", i)
		}
		if err == nil {
			t.Errorf("#%d: unexpected nil err", i)
		}
	}
}

func TestNewProxyFunc(t *testing.T) {
	tests := map[string]string{
		"bar.com":              "http://bar.com",
		"http://disco.foo.bar": "http://disco.foo.bar",
	}
	for in, w := range tests {
		pf, err := newProxyFunc(in)
		if pf == nil {
			t.Errorf("%s: unexpected nil proxyFunc", in)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected non-nil err: %v", in, err)
			continue
		}
		g, err := pf(&http.Request{})
		if err != nil {
			t.Errorf("%s: unexpected non-nil err: %v", in, err)
		}
		if g.String() != w {
			t.Errorf("%s: proxyURL=%q, want %q", in, g, w)
		}

	}
}

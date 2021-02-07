// Copyright 2021 The etcd Authors
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

package endpoint

import (
	"testing"
)

func TestInterpret(t *testing.T) {
	tests := []struct {
		endpoint       string
		wantAddress    string
		wantServerName string
	}{
		{"127.0.0.1", "127.0.0.1", "127.0.0.1"},
		{"localhost", "localhost", "localhost"},
		{"localhost:8080", "localhost:8080", "localhost"},

		{"unix:127.0.0.1", "unix:127.0.0.1", "127.0.0.1"},
		{"unix:127.0.0.1:8080", "unix:127.0.0.1:8080", "127.0.0.1"},

		{"unix://127.0.0.1", "unix:127.0.0.1", "127.0.0.1"},
		{"unix://127.0.0.1:8080", "unix:127.0.0.1:8080", "127.0.0.1"},

		{"unixs:127.0.0.1", "unix:127.0.0.1", "127.0.0.1"},
		{"unixs:127.0.0.1:8080", "unix:127.0.0.1:8080", "127.0.0.1"},
		{"unixs://127.0.0.1", "unix:127.0.0.1", "127.0.0.1"},
		{"unixs://127.0.0.1:8080", "unix:127.0.0.1:8080", "127.0.0.1"},

		{"http://127.0.0.1", "127.0.0.1", "127.0.0.1"},
		{"http://127.0.0.1:8080", "127.0.0.1:8080", "127.0.0.1"},
		{"https://127.0.0.1", "127.0.0.1", "127.0.0.1"},
		{"https://127.0.0.1:8080", "127.0.0.1:8080", "127.0.0.1"},
		{"https://localhost:20000", "localhost:20000", "localhost"},

		{"unix:///tmp/abc", "unix:///tmp/abc", "/tmp/abc"},
		{"unixs:///tmp/abc", "unix:///tmp/abc", "/tmp/abc"},
		{"etcd.io", "etcd.io", "etcd.io"},
		{"http://etcd.io/abc", "etcd.io", "etcd.io"},
		{"dns://something-other", "dns://something-other", "dns://something-other"},
	}
	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			gotAddress, gotServerName := Interpret(tt.endpoint)
			if gotAddress != tt.wantAddress {
				t.Errorf("Interpret() gotAddress = %v, want %v", gotAddress, tt.wantAddress)
			}
			if gotServerName != tt.wantServerName {
				t.Errorf("Interpret() gotServerName = %v, want %v", gotServerName, tt.wantServerName)
			}
		})
	}
}

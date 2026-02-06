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

func Test_interpret(t *testing.T) {
	tests := []struct {
		endpoint          string
		wantAddress       string
		wantServerName    string
		wantRequiresCreds CredsRequirement
	}{
		{"127.0.0.1", "127.0.0.1", "127.0.0.1", CredsOptional},
		{"localhost", "localhost", "localhost", CredsOptional},
		{"localhost:8080", "localhost:8080", "localhost:8080", CredsOptional},

		{"unix:127.0.0.1", "unix:127.0.0.1", "127.0.0.1", CredsOptional},
		{"unix:127.0.0.1:8080", "unix:127.0.0.1:8080", "127.0.0.1:8080", CredsOptional},

		{"unix://127.0.0.1", "unix:127.0.0.1", "127.0.0.1", CredsOptional},
		{"unix://127.0.0.1:8080", "unix:127.0.0.1:8080", "127.0.0.1:8080", CredsOptional},

		{"unixs:127.0.0.1", "unix:127.0.0.1", "127.0.0.1", CredsRequire},
		{"unixs:127.0.0.1:8080", "unix:127.0.0.1:8080", "127.0.0.1:8080", CredsRequire},
		{"unixs://127.0.0.1", "unix:127.0.0.1", "127.0.0.1", CredsRequire},
		{"unixs://127.0.0.1:8080", "unix:127.0.0.1:8080", "127.0.0.1:8080", CredsRequire},

		{"http://127.0.0.1", "127.0.0.1", "127.0.0.1", CredsDrop},
		{"http://127.0.0.1:8080", "127.0.0.1:8080", "127.0.0.1:8080", CredsDrop},
		{"https://127.0.0.1", "127.0.0.1", "127.0.0.1", CredsRequire},
		{"https://127.0.0.1:8080", "127.0.0.1:8080", "127.0.0.1:8080", CredsRequire},
		{"https://localhost:20000", "localhost:20000", "localhost:20000", CredsRequire},

		{"unix:///tmp/abc", "unix:///tmp/abc", "abc", CredsOptional},
		{"unixs:///tmp/abc", "unix:///tmp/abc", "abc", CredsRequire},
		{"unix:///tmp/abc:1234", "unix:///tmp/abc:1234", "abc:1234", CredsOptional},
		{"unixs:///tmp/abc:1234", "unix:///tmp/abc:1234", "abc:1234", CredsRequire},
		{"etcd.io", "etcd.io", "etcd.io", CredsOptional},
		{"http://etcd.io/abc", "etcd.io", "etcd.io", CredsDrop},
		{"dns://something-other", "dns://something-other", "something-other", CredsOptional},

		{"http://[2001:db8:1f70::999:de8:7648:6e8]:100/", "[2001:db8:1f70::999:de8:7648:6e8]:100", "[2001:db8:1f70::999:de8:7648:6e8]:100", CredsDrop},
		{"[2001:db8:1f70::999:de8:7648:6e8]:100", "[2001:db8:1f70::999:de8:7648:6e8]:100", "[2001:db8:1f70::999:de8:7648:6e8]:100", CredsOptional},
		{"unix:unexpected-file_name#123$456", "unix:unexpected-file_name#123$456", "unexpected-file_name#123$456", CredsOptional},
	}
	for _, tt := range tests {
		t.Run("Interpret_"+tt.endpoint, func(t *testing.T) {
			gotAddress, gotServerName := Interpret(tt.endpoint)
			if gotAddress != tt.wantAddress {
				t.Errorf("Interpret() gotAddress = %v, want %v", gotAddress, tt.wantAddress)
			}
			if gotServerName != tt.wantServerName {
				t.Errorf("Interpret() gotServerName = %v, want %v", gotServerName, tt.wantServerName)
			}
		})
		t.Run("RequiresCredentials_"+tt.endpoint, func(t *testing.T) {
			requiresCreds := RequiresCredentials(tt.endpoint)
			if requiresCreds != tt.wantRequiresCreds {
				t.Errorf("RequiresCredentials() got = %v, want %v", requiresCreds, tt.wantRequiresCreds)
			}
		})
	}
}

func Test_extractHostFromHostPort(t *testing.T) {
	tests := []struct {
		ep   string
		want string
	}{
		{ep: "localhost", want: "localhost"},
		{ep: "localhost:8080", want: "localhost"},
		{ep: "192.158.7.14:8080", want: "192.158.7.14"},
		{ep: "192.158.7.14:8080", want: "192.158.7.14"},
		{ep: "[2001:db8:1f70::999:de8:7648:6e8]", want: "[2001:db8:1f70::999:de8:7648:6e8]"},
		{ep: "[2001:db8:1f70::999:de8:7648:6e8]:100", want: "2001:db8:1f70::999:de8:7648:6e8"},
	}
	for _, tt := range tests {
		t.Run(tt.ep, func(t *testing.T) {
			if got := extractHostFromHostPort(tt.ep); got != tt.want {
				t.Errorf("extractHostFromHostPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

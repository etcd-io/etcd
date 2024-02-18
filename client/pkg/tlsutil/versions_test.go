// Copyright 2023 The etcd Authors
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

package tlsutil

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		want        uint16
		expectError bool
	}{
		{
			name:    "TLS1.2",
			version: "TLS1.2",
			want:    tls.VersionTLS12,
		},
		{
			name:    "TLS1.3",
			version: "TLS1.3",
			want:    tls.VersionTLS13,
		},
		{
			name:    "Empty version",
			version: "",
			want:    0,
		},
		{
			name:        "Converting invalid version string to TLS version",
			version:     "not_existing",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTLSVersion(tt.version)
			if err != nil {
				assert.True(t, tt.expectError, "GetTLSVersion() returned error while expecting success: %v", err)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

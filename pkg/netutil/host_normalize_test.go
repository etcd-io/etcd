// Copyright 2025 The etcd Authors
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

package netutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestIPv6AddressNormalization(t *testing.T) {
	testCases := []struct {
		name     string
		urlsA    []string
		urlsB    []string
		expected bool
	}{
		{
			name:     "IPv6 with leading zeros vs without should match",
			urlsA:    []string{"https://[c262:266f:fa53:0ee6:966e:e3f0:d68f:b046]:2380"},
			urlsB:    []string{"https://[c262:266f:fa53:ee6:966e:e3f0:d68f:b046]:2380"},
			expected: true,
		},
		{
			name:     "IPv6 different (lower/upper) case should match",
			urlsA:    []string{"https://[2001:DB8::1]:2380"},
			urlsB:    []string{"https://[2001:db8::1]:2380"},
			expected: true,
		},
		{
			name:     "IPv4 address normalization should still work",
			urlsA:    []string{"http://192.168.1.1:2380"},
			urlsB:    []string{"http://192.168.1.1:2380"},
			expected: true,
		},
		{
			name:     "Different IPv6 addresses should not match",
			urlsA:    []string{"https://[2001:db8::1]:2380"},
			urlsB:    []string{"https://[2001:db8::2]:2380"},
			expected: false,
		},
		{
			name:     "IPv6 without port should match",
			urlsA:    []string{"https://[2001:db8::1]"},
			urlsB:    []string{"https://[2001:0db8:0:00:000:0000:0:01]"},
			expected: true,
		},
		{
			name:     "IPv4 without port should match",
			urlsA:    []string{"http://192.168.1.1"},
			urlsB:    []string{"http://192.168.1.1"},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := URLStringsEqual(t.Context(), zaptest.NewLogger(t), tc.urlsA, tc.urlsB)
			if tc.expected {
				assert.True(t, result)
			} else {
				if err != nil {
					t.Logf("Got expected error for non-matching URLs: %v", err)
				} else {
					assert.False(t, result)
				}
			}
		})
	}
}

func TestNormalizeHostFunction(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "IPv6 with leading zeros should be normalized",
			input:    "[c262:266f:fa53:0ee6:966e:e3f0:d68f:b046]:2380",
			expected: "[c262:266f:fa53:ee6:966e:e3f0:d68f:b046]:2380",
		},
		{
			name:     "Compressed IPv6 should remain compressed",
			input:    "[2001:db8::1]:2380",
			expected: "[2001:db8::1]:2380",
		},
		{
			name:     "IPv6 case should be normalized to lowercase",
			input:    "[2001:DB8::1]:2380",
			expected: "[2001:db8::1]:2380",
		},
		{
			name:     "IPv4 should remain unchanged",
			input:    "192.168.1.1:2380",
			expected: "192.168.1.1:2380",
		},
		{
			name:     "Hostname should remain unchanged",
			input:    "example.com:2380",
			expected: "example.com:2380",
		},
		{
			name:     "IPv6 without port",
			input:    "[2025:db8::1]",
			expected: "[2025:db8::1]",
		},
		{
			name:     "IPv4 without port",
			input:    "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "Hostname without port",
			input:    "example.com",
			expected: "example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeHost(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

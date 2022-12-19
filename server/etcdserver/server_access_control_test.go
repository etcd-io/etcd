// Copyright 2022 The etcd Authors
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

package etcdserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		accessController *AccessController
		origin           string
		allowed          bool
	}{
		{
			&AccessController{
				CORS: map[string]struct{}{},
			},
			"https://example.com",
			true,
		},
		{
			&AccessController{
				CORS: map[string]struct{}{"*": {}},
			},
			"https://example.com",
			true,
		},
		{
			&AccessController{
				CORS: map[string]struct{}{"https://example.com": {}, "http://example.org": {}},
			},
			"https://example.com",
			true,
		},
		{
			&AccessController{
				CORS: map[string]struct{}{"http://example.org": {}},
			},
			"https://example.com",
			false,
		},
		{
			&AccessController{
				CORS: map[string]struct{}{"*": {}, "http://example.org/": {}},
			},
			"https://example.com",
			true,
		},
	}

	for _, tt := range tests {
		allowed := tt.accessController.OriginAllowed(tt.origin)
		assert.Equal(t, allowed, tt.allowed)
	}
}

func TestIsHostWhitelisted(t *testing.T) {
	tests := []struct {
		accessController *AccessController
		host             string
		whitelisted      bool
	}{
		{
			&AccessController{
				HostWhitelist: map[string]struct{}{},
			},
			"example.com",
			true,
		},
		{
			&AccessController{
				HostWhitelist: map[string]struct{}{"*": {}},
			},
			"example.com",
			true,
		},
		{
			&AccessController{
				HostWhitelist: map[string]struct{}{"example.com": {}, "example.org": {}},
			},
			"example.com",
			true,
		},
		{
			&AccessController{
				HostWhitelist: map[string]struct{}{"example.org": {}},
			},
			"example.com",
			false,
		},
		{
			&AccessController{
				HostWhitelist: map[string]struct{}{"*": {}, "example.org/": {}},
			},
			"example.com",
			true,
		},
	}

	for _, tt := range tests {
		whitelisted := tt.accessController.IsHostWhitelisted(tt.host)
		assert.Equal(t, whitelisted, tt.whitelisted)
	}
}

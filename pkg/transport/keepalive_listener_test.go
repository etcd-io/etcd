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

package transport

import (
	"net/http"
	"testing"
)

// TestNewKeepAliveListener tests NewKeepAliveListener returns a listener
// that accepts connections.
// TODO: verify the keepalive option is set correctly
func TestNewKeepAliveListener(t *testing.T) {
	ln, err := NewKeepAliveListener("127.0.0.1:0", "http", TLSInfo{})
	if err != nil {
		t.Fatalf("unexpected NewKeepAliveListener error: %v", err)
	}

	go http.Get("http://" + ln.Addr().String())
	conn, err := ln.Accept()
	if err != nil {
		t.Fatalf("unexpected Accept error: %v", err)
	}
	conn.Close()
}

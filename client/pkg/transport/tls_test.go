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

package transport

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateSecureEndpoints(t *testing.T) {
	tlsInfo, err := createSelfCert(t)
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}

	remoteAddr := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.RemoteAddr))
	}
	srv := httptest.NewServer(http.HandlerFunc(remoteAddr))
	defer srv.Close()

	insecureEps := []string{
		"http://" + srv.Listener.Addr().String(),
		"invalid remote address",
	}
	if _, err := ValidateSecureEndpoints(*tlsInfo, insecureEps); err == nil || !strings.Contains(err.Error(), "is insecure") {
		t.Error("validate secure endpoints should fail")
	}

	secureEps := []string{
		"https://" + srv.Listener.Addr().String(),
	}
	if _, err := ValidateSecureEndpoints(*tlsInfo, secureEps); err != nil {
		t.Error("validate secure endpoints should succeed")
	}
}

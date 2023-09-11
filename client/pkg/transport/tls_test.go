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
	"reflect"
	"testing"
)

func TestValidateSecureEndpoints(t *testing.T) {
	tlsInfo, certCleanup, err := createSelfCert()
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}
	defer certCleanup()

	remoteAddr := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.RemoteAddr))
	}
	srv := httptest.NewServer(http.HandlerFunc(remoteAddr))
	defer srv.Close()

	tests := map[string]struct {
		endPoints         []string
		expectedEndpoints []string
		expectedErr       bool
	}{
		"invalidEndPoints": {
			endPoints: []string{
				"invalid endpoint",
			},
			expectedEndpoints: nil,
			expectedErr:       true,
		},
		"insecureEndpoints": {
			endPoints: []string{
				"http://127.0.0.1:8000",
				"http://" + srv.Listener.Addr().String(),
			},
			expectedEndpoints: nil,
			expectedErr:       true,
		},
		"secureEndPoints": {
			endPoints: []string{
				"https://" + srv.Listener.Addr().String(),
			},
			expectedEndpoints: []string{
				"https://" + srv.Listener.Addr().String(),
			},
			expectedErr: false,
		},
		"mixEndPoints": {
			endPoints: []string{
				"https://" + srv.Listener.Addr().String(),
				"http://" + srv.Listener.Addr().String(),
				"invalid end points",
			},
			expectedEndpoints: []string{
				"https://" + srv.Listener.Addr().String(),
			},
			expectedErr: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			secureEps, err := ValidateSecureEndpoints(*tlsInfo, test.endPoints)
			if test.expectedErr != (err != nil) {
				t.Errorf("Unexpected error, got: %v, want: %v", err, test.expectedErr)
			}

			if !reflect.DeepEqual(test.expectedEndpoints, secureEps) {
				t.Errorf("expected endpoints %v, got %v", test.expectedEndpoints, secureEps)
			}
		})
	}
}

// Copyright 2015 The etcd Authors
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

package v2error

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorWriteTo(t *testing.T) {
	for k := range errors {
		err := NewError(k, "", 1)
		rr := httptest.NewRecorder()
		err.WriteTo(rr)

		assert.Equalf(t, err.StatusCode(), rr.Code, "HTTP status code %d, want %d", rr.Code, err.StatusCode())

		gbody := strings.TrimSuffix(rr.Body.String(), "\n")
		assert.Equalf(t, err.toJSONString(), gbody, "HTTP body %q, want %q", gbody, err.toJSONString())

		wheader := http.Header(map[string][]string{
			"Content-Type": {"application/json"},
			"X-Etcd-Index": {"1"},
		})

		assert.Truef(t, reflect.DeepEqual(wheader, rr.HeaderMap), "HTTP headers %v, want %v", rr.HeaderMap, wheader)
	}
}

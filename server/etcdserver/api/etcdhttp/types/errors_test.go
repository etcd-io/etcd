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

package httptypes

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPErrorWriteTo(t *testing.T) {
	err := NewHTTPError(http.StatusBadRequest, "what a bad request you made!")
	rr := httptest.NewRecorder()
	e := err.WriteTo(rr)
	require.NoErrorf(t, e, "HTTPError.WriteTo error (%v)", e)

	wcode := http.StatusBadRequest
	wheader := http.Header(map[string][]string{
		"Content-Type": {"application/json"},
	})
	wbody := `{"message":"what a bad request you made!"}`

	assert.Equalf(t, wcode, rr.Code, "HTTP status code %d, want %d", rr.Code, wcode)

	assert.Truef(t, reflect.DeepEqual(wheader, rr.HeaderMap), "HTTP headers %v, want %v", rr.HeaderMap, wheader)

	gbody := rr.Body.String()
	assert.Equalf(t, wbody, gbody, "HTTP body %q, want %q", gbody, wbody)
}

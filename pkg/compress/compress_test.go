// Copyright 2016 CoreOS, Inc.
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

package compress

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseType(t *testing.T) {
	if tp := ParseType("aa"); tp != NoCompress {
		t.Fatalf("expected NoCompress, got %v", tp)
	}

	if tp := ParseType("gzip"); tp != Gzip {
		t.Fatalf("expected Gzip, got %v", tp)
	} else if tp.String() != "gzip" {
		t.Fatalf("expected gzip, got %s", tp)
	}

	if tp := ParseType("snappy"); tp != Snappy {
		t.Fatalf("expected Snappy, got %v", tp)
	} else if tp.String() != "snappy" {
		t.Fatalf("expected snappy, got %s", tp)
	}
}

func TestCompress(t *testing.T) {
	var testTxt = bytes.Repeat([]byte("a"), 1024)

	mainRouter := http.NewServeMux()
	mainRouter.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		rw.Write(testTxt)
	})
	srv := httptest.NewServer(NewHandler(mainRouter))
	defer srv.Close()

	for _, algo := range []Type{NoCompress, Gzip, Snappy} {
		req, err := http.NewRequest("GET", srv.URL, nil)
		if err != nil {
			t.Fatalf("%s: %v", algo, err)
		}
		response, err := http.DefaultClient.Do(NewRequest(req, algo))
		if err != nil {
			t.Fatalf("%s: %v", algo, err)
		}

		resp := NewResponseReader(response)
		defer resp.Close()

		decoded, err := ioutil.ReadAll(resp)
		if err != nil {
			t.Fatalf("%s: %v", algo, err)
		}
		if !bytes.Equal(testTxt, decoded) {
			t.Fatalf("%s: expected %q, got %q", algo, testTxt, decoded)
		}
	}
}

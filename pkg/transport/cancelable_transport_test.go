// Copyright 2017 The etcd Authors
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
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCancelableTransportCancel(t *testing.T) {
	sock := "whatever:123"
	l, err := NewUnixListener(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	tr, trerr := NewTransport(TLSInfo{}, time.Second)
	if trerr != nil {
		t.Fatal(trerr)
	}
	tr.Cancel()

	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		req, reqerr := http.NewRequest("GET", "unix://"+sock, strings.NewReader("abc"))
		if reqerr != nil {
			errc <- reqerr
			return
		}
		resp, rerr := tr.RoundTrip(req)
		if rerr == nil {
			errc <- fmt.Errorf("round trip succeeded with %+v, expected error", resp)
		}
	}()

	select {
	case err := <-errc:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for roundtrip to cancel")
	}
}

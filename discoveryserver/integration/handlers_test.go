// Copyright 2019 The etcd Authors
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

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.etcd.io/etcd/client"
)

var basePort int32 = 10000

func TestHandlersV2_size_1(t *testing.T)   { testHandlersV2(t, 1) }
func TestHandlersV2_size_3(t *testing.T)   { testHandlersV2(t, 3) }
func TestHandlersV2_size_5(t *testing.T)   { testHandlersV2(t, 5) }
func TestHandlersV2_size_7(t *testing.T)   { testHandlersV2(t, 7) }
func TestHandlersV2_size_10(t *testing.T)  { testHandlersV2(t, 10) }
func TestHandlersV2_size_100(t *testing.T) { testHandlersV2(t, 100) }
func testHandlersV2(t *testing.T, size int) {
	cport := int(atomic.LoadInt32(&basePort))
	atomic.AddInt32(&basePort, int32(5))

	svs := NewService(t, cport, cport+1, cport+2)
	defer svs.Stop(t)

	errc := svs.Start(t)
	select {
	case err := <-errc:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		// wait for http listeners to start
		// (slow CI machines often take a few seconds)
	}

	// check health endpoint
	resp, err := http.Get(svs.httpEp + "/health")
	if err != nil {
		t.Fatal(err)
	}
	bts, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	gracefulClose(resp)
	if !bytes.Equal(bts, []byte("OK")) {
		t.Fatalf("expected 'OK', got %q", string(bts))
	}

	// create a token
	resp, err = http.Get(svs.httpEp + fmt.Sprintf("/new?size=%d", size))
	if err != nil {
		t.Fatal(err)
	}
	bts, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	gracefulClose(resp)
	hostToken := string(bts)
	if !strings.HasPrefix(hostToken, testDiscoveryHost+"/") {
		t.Fatalf("expected %q in prefix, got %q", testDiscoveryHost, hostToken)
	}
	token := strings.Replace(hostToken, testDiscoveryHost+"/", "", 1)
	if !isAlphanumeric(token) {
		t.Fatalf("token %q is not alphanumeric", token)
	}
	if len(token) != 32 {
		t.Fatalf("token %q must be 32-character", token)
	}

	var cresp client.Response

	// query the token
	for i, p := range []string{fmt.Sprintf("/%s", token), fmt.Sprintf("/%s/", token)} {
		resp, err = http.Get(svs.httpEp + p)
		if err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
		if err = json.NewDecoder(resp.Body).Decode(&cresp); err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
		gracefulClose(resp)
		if cresp.Node == nil {
			t.Fatalf("#%d: token response returned <nil> node", i)
		}
		exp := "/" + path.Join(token)
		if !(cresp.Node.Key == exp || cresp.Node.Key == (exp+"/")) {
			t.Fatalf("key expected %q, got %q", exp, cresp.Node.Key)
		}
		if !cresp.Node.Dir {
			t.Fatalf("#%d: node expected directory, got dir %v", i, cresp.Node.Dir)
		}
		if len(cresp.Node.Nodes) > 0 {
			t.Fatalf("#%d: unexpected cluster members found, got %+v", i, cresp.Node.Nodes)
		}
		// index must have increased after health check
		if cresp.Node.CreatedIndex != 5 {
			t.Fatalf("cresp.Node.CreatedIndex expected 5, got %d", cresp.Node.CreatedIndex)
		}
		if cresp.Node.ModifiedIndex != 5 {
			t.Fatalf("cresp.Node.ModifiedIndex expected 5, got %d", cresp.Node.ModifiedIndex)
		}
	}

	// query the size
	resp, err = http.Get(svs.httpEp + fmt.Sprintf("/%s/_config/size", token))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.NewDecoder(resp.Body).Decode(&cresp); err != nil {
		t.Fatal(err)
	}
	gracefulClose(resp)
	if cresp.Node == nil {
		t.Fatal("token response returned <nil> node")
	}
	if !strings.HasSuffix(cresp.Node.Key, token+"/_config/size") {
		t.Fatalf("node key is missing '/_config/size' in %q, got %q", token, cresp.Node.Key)
	}
	if cresp.Node.Value != fmt.Sprintf("%d", size) {
		t.Fatalf("size expected %d, got %s", size, cresp.Node.Value)
	}

	// simulate PUT from etcd servers to discovery server
	// just as v2 PUT 'curl http://127.0.0.1:2379/v2/keys/foo -XPUT -d value=bar'
	// 'curl http://127.0.0.1:2379/v2/keys/foo'
	for i := 0; i < size; i++ {
		memberID := fmt.Sprintf("id%d", i)
		form := url.Values{}
		form.Add("value", fmt.Sprintf("%s=http://test.com:%d", memberID, i))

		req, err := http.NewRequest(http.MethodPut, svs.httpEp+fmt.Sprintf("/%s", path.Join(token, memberID)), strings.NewReader(form.Encode()))

		if err != nil {
			t.Fatal(err)
		}
		_, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}

	}

	for i, ep := range []string{
		svs.httpEp + fmt.Sprintf("/%s", token),
	} {
		var cresp client.Response
		t.Logf("endpoint: %v\n\n", ep)
		resp, err = http.Get(ep)
		if err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
		if err = json.NewDecoder(resp.Body).Decode(&cresp); err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
		gracefulClose(resp)
		t.Logf("#%d: token response returned %v", i, cresp)
		if cresp.Node == nil {
			t.Fatalf("#%d: token response returned <nil> node", i)
		}
		exp := "/" + token
		if cresp.Node.Key != exp {
			t.Logf("#%d: key expected %q, got %q", i, exp, cresp.Node.Key)
			t.Fatalf("#%d: key expected %q, got %q", i, exp, cresp.Node.Key)
		}
		if !cresp.Node.Dir {
			t.Fatalf("#%d: node expected directory, got dir %v", i, cresp.Node.Dir)
		}
		if len(cresp.Node.Nodes) != size {
			t.Fatalf("#%d: expected %d cluster members found, got %+v", i, size, cresp.Node.Nodes)
		}
	}

	time.Sleep(3 * time.Second)
	svs.state.GarbageCollect(time.Second, time.Minute)

	// query the size
	resp, err = http.Get(svs.httpEp + fmt.Sprintf("/%s/_config/size", token))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expecting 400 after GC got %v", resp.StatusCode)
	}

}

var isAlphanumeric = regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString

func gracefulClose(resp *http.Response) {
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
}

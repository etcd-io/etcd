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

package embed

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"testing"

	"go.etcd.io/etcd/server/v3/auth"
)

// TestStartEtcdWrongToken ensures that StartEtcd with wrong configs returns with error.
func TestStartEtcdWrongToken(t *testing.T) {
	tdir, err := ioutil.TempDir(t.TempDir(), "token-test")

	if err != nil {
		t.Fatal(err)
	}

	cfg := NewConfig()

	// Similar to function in integration/embed/embed_test.go for setting up Config.
	urls := newEmbedURLs(2)
	curls := []url.URL{urls[0]}
	purls := []url.URL{urls[1]}
	cfg.LCUrls, cfg.ACUrls = curls, curls
	cfg.LPUrls, cfg.APUrls = purls, purls
	cfg.InitialCluster = ""
	for i := range purls {
		cfg.InitialCluster += ",default=" + purls[i].String()
	}
	cfg.InitialCluster = cfg.InitialCluster[1:]
	cfg.Dir = tdir
	cfg.AuthToken = "wrong-token"

	if _, err = StartEtcd(cfg); err != auth.ErrInvalidAuthOpts {
		t.Fatalf("expected %v, got %v", auth.ErrInvalidAuthOpts, err)
	}
}

func newEmbedURLs(n int) (urls []url.URL) {
	scheme := "unix"
	for i := 0; i < n; i++ {
		u, _ := url.Parse(fmt.Sprintf("%s://localhost:%d%06d", scheme, os.Getpid(), i))
		urls = append(urls, *u)
	}
	return urls
}

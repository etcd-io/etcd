// Copyright 2026 The etcd Authors
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

package testutil

import (
	"fmt"
	"net/url"
	"os"
)

type ConfigTestURLs struct {
	PeerURLs       []url.URL
	ClientURLs     []url.URL
	InitialCluster string
}

// Similar to function in integration/embed/embed_test.go for setting up Config.
func NewConfigTestURLs() *ConfigTestURLs {
	urls := newEmbedURLs(2)
	cfg := &ConfigTestURLs{
		ClientURLs:     []url.URL{urls[0]},
		PeerURLs:       []url.URL{urls[1]},
		InitialCluster: "",
	}
	for i := range cfg.PeerURLs {
		cfg.InitialCluster += ",default=" + cfg.PeerURLs[i].String()
	}
	cfg.InitialCluster = cfg.InitialCluster[1:]
	return cfg
}

func newEmbedURLs(n int) (urls []url.URL) {
	scheme := "unix"
	for i := 0; i < n; i++ {
		u, _ := url.Parse(fmt.Sprintf("%s://localhost:%d%06d", scheme, os.Getpid(), i))
		urls = append(urls, *u)
	}
	return urls
}

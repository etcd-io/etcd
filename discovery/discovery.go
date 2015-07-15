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

package discovery

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"
	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/types"
)

var (
	plog = capnslog.NewPackageLogger("github.com/coreos/etcd", "discovery")
)

// JoinCluster will connect to the discovery service at the given url, and
// register the server represented by the given id and config to the cluster
func JoinCluster(durl, dproxyurl string, id types.ID, config string) (string, error) {
	d, err := newDiscovery(durl, dproxyurl, id)
	if err != nil {
		return "", err
	}
	return d.JoinCluster(config)
}

// GetCluster will connect to the discovery service at the given url and
// retrieve a string describing the cluster
func GetCluster(durl, dproxyurl string) (string, error) {
	d, err := newDiscovery(durl, dproxyurl, 0)
	if err != nil {
		return "", err
	}
	return d.GetCluster()
}

// newProxyFunc builds a proxy function from the given string, which should
// represent a URL that can be used as a proxy. It performs basic
// sanitization of the URL and returns any error encountered.
func newProxyFunc(proxy string) (func(*http.Request) (*url.URL, error), error) {
	if proxy == "" {
		return nil, nil
	}
	// Do a small amount of URL sanitization to help the user
	// Derived from net/http.ProxyFromEnvironment
	proxyURL, err := url.Parse(proxy)
	if err != nil || !strings.HasPrefix(proxyURL.Scheme, "http") {
		// proxy was bogus. Try prepending "http://" to it and
		// see if that parses correctly. If not, we ignore the
		// error and complain about the original one
		var err2 error
		proxyURL, err2 = url.Parse("http://" + proxy)
		if err2 == nil {
			err = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid proxy address %q: %v", proxy, err)
	}

	plog.Infof("using proxy %q", proxyURL.String())
	return http.ProxyURL(proxyURL), nil
}

func newDiscovery(durl, dproxyurl string, id types.ID) (client.DiscoveryService, error) {
	u, err := url.Parse(durl)
	if err != nil {
		return nil, err
	}
	token := u.Path
	u.Path = ""
	pf, err := newProxyFunc(dproxyurl)
	if err != nil {
		return nil, err
	}
	cfg := client.Config{
		Transport: &http.Transport{
			Proxy: pf,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
		},
		Endpoints: []string{u.String()},
	}
	c, err := client.New(cfg)
	if err != nil {
		return nil, err
	}
	return client.NewDiscoveryService(c, token, id, u), nil
}

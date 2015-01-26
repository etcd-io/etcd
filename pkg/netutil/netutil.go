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

package netutil

import (
	"log"
	"net"
	"net/url"
	"reflect"
)

var (
	// indirection for testing
	resolveTCPAddr = net.ResolveTCPAddr
)

// ResolveTCPAddrs is a convenience wrapper for net.ResolveTCPAddr.
// ResolveTCPAddrs resolves all DNS hostnames in-place for the given set of
// url.URLs.
func ResolveTCPAddrs(urls ...[]url.URL) error {
	for _, us := range urls {
		for i, u := range us {
			host, _, err := net.SplitHostPort(u.Host)
			if err != nil {
				log.Printf("netutil: Could not parse url %s during tcp resolving.", u.Host)
				return err
			}
			if host == "localhost" {
				continue
			}
			if net.ParseIP(host) != nil {
				continue
			}
			tcpAddr, err := resolveTCPAddr("tcp", u.Host)
			if err != nil {
				log.Printf("netutil: Could not resolve host: %s", u.Host)
				return err
			}
			log.Printf("netutil: Resolving %s to %s", u.Host, tcpAddr.String())
			us[i].Host = tcpAddr.String()
		}
	}
	return nil
}

// URLsEqual checks equality of url.URLS between two arrays.
// This check pass even if an URL is in hostname and opposite is in IP address.
func URLsEqual(a []url.URL, b []url.URL) bool {
	if len(a) != len(b) {
		return false
	}
	for i, urlA := range a {
		urlB := b[i]

		if !reflect.DeepEqual(urlA, urlB) {
			urls := []url.URL{urlA, urlB}
			ResolveTCPAddrs(urls)
			if !reflect.DeepEqual(urls[0], urls[1]) {
				return false
			}
		}
	}

	return true
}

func URLStringsEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	urlsA := make([]url.URL, len(a))
	for _, str := range a {
		u, err := url.Parse(str)
		if err != nil {
			return false
		}
		urlsA = append(urlsA, *u)
	}
	urlsB := make([]url.URL, len(b))
	for _, str := range b {
		u, err := url.Parse(str)
		if err != nil {
			return false
		}
		urlsB = append(urlsB, *u)
	}

	return URLsEqual(urlsA, urlsB)
}
